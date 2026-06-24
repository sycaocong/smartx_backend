package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// 订单对象池（减少GC压力）
var orderPool = sync.Pool{
	New: func() interface{} {
		return &Order{}
	},
}

// Trade对象池
var tradePool = sync.Pool{
	New: func() interface{} {
		return &Trade{}
	},
}

// MatchingEngine 撮合引擎
type MatchingEngine struct {
	symbol    string
	orderBook *OrderBook
	tradeCh   chan *Trade // 成交通道
	orderCh   chan *Order // 订单通道
	doneCh    chan struct{}

	// 统计
	processedCount int64
	failedCount    int64

	// 日志
	logger zerolog.Logger

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup
}

// NewMatchingEngine 创建撮合引擎
func NewMatchingEngine(symbol string, logger zerolog.Logger) *MatchingEngine {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &MatchingEngine{
		symbol:    symbol,
		orderBook: NewOrderBook(symbol),
		tradeCh:   make(chan *Trade, 50000), // 增大buffer提高吞吐量
		orderCh:   make(chan *Order, 50000), // 增大buffer提高吞吐量
		doneCh:    make(chan struct{}),
		logger:    logger.With().Str("symbol", symbol).Logger(),
		ctx:       ctx,
		cancel:    cancel,
	}

	// 启动撮合协程
	engine.wg.Add(1)
	go engine.run()

	return engine
}

// run 撮合引擎主循环
func (e *MatchingEngine) run() {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		case order := <-e.orderCh:
			e.processOrder(order)
		}
	}
}

// processOrder 处理订单
func (e *MatchingEngine) processOrder(order *Order) {
	atomic.AddInt64(&e.processedCount, 1)

	e.logger.Debug().
		Str("order_id", order.OrderID).
		Float64("price", order.Price).
		Float64("quantity", order.Quantity).
		Str("side", func() string {
			if order.Side == Buy {
				return "BUY"
			}
			return "SELL"
		}()).
		Msg("Processing order")

	// 市价单直接成交
	if order.Type == MarketOrder {
		e.processMarketOrder(order)
		return
	}

	// 限价单撮合
	trades := e.orderBook.Match(order)

	if len(trades) > 0 {
		e.logger.Info().
			Int("trade_count", len(trades)).
			Float64("total_quantity", e.calculateTotalQuantity(trades)).
			Msg("Order matched")

		// 发送成交记录
		for _, trade := range trades {
			select {
			case e.tradeCh <- trade:
			default:
				e.logger.Warn().Str("trade_id", trade.TradeID).Msg("Trade channel full, dropping trade")
			}
		}
	}

	// 如果订单未完全成交，添加到订单簿
	if !order.IsDone() && order.Status != StatusCanceled {
		e.orderBook.AddOrder(order)
		e.logger.Debug().
			Float64("remaining", order.RemainingQuantity()).
			Msg("Order added to book")
	}
}

// processMarketOrder 处理市价单
func (e *MatchingEngine) processMarketOrder(order *Order) {
	trades := e.orderBook.Match(order)

	if len(trades) > 0 {
		for _, trade := range trades {
			select {
			case e.tradeCh <- trade:
			default:
				e.logger.Warn().Str("trade_id", trade.TradeID).Msg("Trade channel full, dropping trade")
			}
		}
	} else {
		// 市价单无法成交，拒绝订单
		order.Status = StatusRejected
		atomic.AddInt64(&e.failedCount, 1)
		e.logger.Warn().Str("order_id", order.OrderID).Msg("Market order rejected - no liquidity")
	}
}

// SubmitOrder 提交订单
func (e *MatchingEngine) SubmitOrder(order *Order) error {
	select {
	case e.orderCh <- order:
		return nil
	case <-time.After(100 * time.Millisecond):
		return ErrOrderChannelFull
	case <-e.ctx.Done():
		return ErrEngineStopped
	}
}

// GetTrades 获取成交记录
func (e *MatchingEngine) GetTrades() []*Trade {
	var trades []*Trade
	for {
		select {
		case trade := <-e.tradeCh:
			trades = append(trades, trade)
		default:
			return trades
		}
	}
}

// GetOrderBook 获取订单簿快照
func (e *MatchingEngine) GetOrderBook() *OrderBookSnapshot {
	bids, asks := e.orderBook.GetDepth(20)

	return &OrderBookSnapshot{
		Symbol:    e.symbol,
		Version:   time.Now().UnixNano(),
		Timestamp: time.Now().UnixMilli(),
		Bids:      bids,
		Asks:      asks,
	}
}

// GetDepth 获取深度
func (e *MatchingEngine) GetDepth(levels int) (bids, asks []PricePoint) {
	return e.orderBook.GetDepth(levels)
}

// calculateTotalQuantity 计算总成交量
func (e *MatchingEngine) calculateTotalQuantity(trades []*Trade) float64 {
	var total float64
	for _, trade := range trades {
		total += trade.Quantity
	}
	return total
}

// Stop 停止引擎
func (e *MatchingEngine) Stop() {
	e.cancel()
	e.wg.Wait()
	close(e.tradeCh)
	close(e.orderCh)
	close(e.doneCh)
}

// GetStats 获取统计信息
func (e *MatchingEngine) GetStats() EngineStats {
	return EngineStats{
		Symbol:         e.symbol,
		ProcessedCount: atomic.LoadInt64(&e.processedCount),
		FailedCount:    atomic.LoadInt64(&e.failedCount),
		TotalBidQty:    e.orderBook.TotalBidQuantity,
		TotalAskQty:    e.orderBook.TotalAskQuantity,
		TradeCount:     e.orderBook.TradeCount,
		LastPrice:      e.orderBook.LastPrice,
		LastTradeTime:  e.orderBook.LastTradeTime,
	}
}

// EngineStats 引擎统计
type EngineStats struct {
	Symbol         string
	ProcessedCount int64
	FailedCount    int64
	TotalBidQty    float64
	TotalAskQty    float64
	TradeCount     int64
	LastPrice      float64
	LastTradeTime  int64
}

// OrderBookSnapshot 订单簿快照
type OrderBookSnapshot struct {
	Symbol    string
	Version   int64
	Timestamp int64
	Bids      []PricePoint
	Asks      []PricePoint
}

// GetBestBidAsk 获取最佳买卖价
func (s *OrderBookSnapshot) GetBestBidAsk() (bestBid, bestAsk float64, bidQty, askQty float64) {
	if len(s.Bids) > 0 {
		bestBid = s.Bids[0].Price
		bidQty = s.Bids[0].Quantity
	}
	if len(s.Asks) > 0 {
		bestAsk = s.Asks[0].Price
		askQty = s.Asks[0].Quantity
	}
	return
}

// 错误定义
var (
	ErrOrderChannelFull = &EngineError{Code: "ORDER_CHANNEL_FULL", Message: "Order channel is full"}
	ErrEngineStopped    = &EngineError{Code: "ENGINE_STOPPED", Message: "Engine has been stopped"}
)

// EngineError 引擎错误
type EngineError struct {
	Code    string
	Message string
}

func (e *EngineError) Error() string {
	return e.Code + ": " + e.Message
}

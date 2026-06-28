package futures

import (
	"context"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// PriceLevel 价格档位结构
// 记录某个价格上的所有订单和总数量
type PriceLevel struct {
	Price    float64        // 价格
	Quantity float64        // 该价格档位总数量
	Orders   []*FuturesOrder // 该价格档位的订单列表
}

// FuturesOrderBook 合约订单簿
// 维护买单和卖单的价格档位，支持撮合匹配
// [Design: 撮合算法](../DESIGN_DERIVATIVES.md#45-撮合算法)
type FuturesOrderBook struct {
	Symbol string              // 交易对
	Bids   map[float64]*PriceLevel // 买单价格档位
	Asks   map[float64]*PriceLevel // 卖单价格档位
	Mu     sync.RWMutex        // 读写锁

	TotalBidQuantity float64 // 买单总数量
	TotalAskQuantity float64 // 卖单总数量
	TradeCount       int64   // 成交次数
	LastPrice        float64 // 最新成交价
	LastTradeTime    int64   // 最新成交时间
}

// NewFuturesOrderBook 创建新的合约订单簿
func NewFuturesOrderBook(symbol string) *FuturesOrderBook {
	return &FuturesOrderBook{
		Symbol: symbol,
		Bids:   make(map[float64]*PriceLevel),
		Asks:   make(map[float64]*PriceLevel),
	}
}

// AddOrder 将订单添加到订单簿
func (ob *FuturesOrderBook) AddOrder(order *FuturesOrder) {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var book map[float64]*PriceLevel
	if order.Side == Buy {
		book = ob.Bids
		ob.TotalBidQuantity += order.Quantity
	} else {
		book = ob.Asks
		ob.TotalAskQuantity += order.Quantity
	}

	if priceLevel, ok := book[order.Price]; ok {
		priceLevel.Quantity += order.Quantity
		priceLevel.Orders = append(priceLevel.Orders, order)
	} else {
		book[order.Price] = &PriceLevel{
			Price:    order.Price,
			Quantity: order.Quantity,
			Orders:   []*FuturesOrder{order},
		}
	}
}

// RemoveOrder 从订单簿移除订单
// 返回: 是否成功移除
func (ob *FuturesOrderBook) RemoveOrder(order *FuturesOrder) bool {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var book map[float64]*PriceLevel
	remainingQty := order.RemainingQuantity()
	if order.Side == Buy {
		book = ob.Bids
		ob.TotalBidQuantity -= remainingQty
	} else {
		book = ob.Asks
		ob.TotalAskQuantity -= remainingQty
	}

	if priceLevel, ok := book[order.Price]; ok {
		priceLevel.Quantity -= remainingQty
		for i, o := range priceLevel.Orders {
			if o.OrderID == order.OrderID {
				priceLevel.Orders = append(priceLevel.Orders[:i], priceLevel.Orders[i+1:]...)
				break
			}
		}
		if priceLevel.Quantity <= 0 || len(priceLevel.Orders) == 0 {
			delete(book, order.Price)
			return true
		}
		return true
	}

	return false
}

// Match 撮合订单
// 采用价格优先、时间优先原则进行订单匹配
// [Design: 撮合算法](../DESIGN_DERIVATIVES.md#45-撮合算法)
func (ob *FuturesOrderBook) Match(incomingOrder *FuturesOrder) []*FuturesTrade {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var trades []*FuturesTrade
	var book map[float64]*PriceLevel

	if incomingOrder.Side == Buy {
		book = ob.Asks
	} else {
		book = ob.Bids
	}

	prices := make([]float64, 0, len(book))
	for price := range book {
		prices = append(prices, price)
	}

	if incomingOrder.Side == Buy {
		for i := 0; i < len(prices); i++ {
			for j := i + 1; j < len(prices); j++ {
				if prices[i] > prices[j] {
					prices[i], prices[j] = prices[j], prices[i]
				}
			}
		}
	} else {
		for i := 0; i < len(prices); i++ {
			for j := i + 1; j < len(prices); j++ {
				if prices[i] < prices[j] {
					prices[i], prices[j] = prices[j], prices[i]
				}
			}
		}
	}

	for _, price := range prices {
		if incomingOrder.IsDone() {
			break
		}

		if incomingOrder.Type != FuturesMarketOrder {
			if incomingOrder.Side == Buy && incomingOrder.Price < price {
				break
			}
			if incomingOrder.Side == Sell && incomingOrder.Price > price {
				break
			}
		}

		priceLevel := book[price]
		if priceLevel == nil {
			continue
		}

		for i := 0; i < len(priceLevel.Orders); i++ {
			if incomingOrder.IsDone() {
				break
			}

			counterOrder := priceLevel.Orders[i]
			if counterOrder.IsDone() {
				continue
			}

			remainingIncoming := incomingOrder.RemainingQuantity()
			remainingCounter := counterOrder.RemainingQuantity()
			matchQuantity := math.Min(remainingIncoming, remainingCounter)

			tradePrice := counterOrder.Price

			incomingOrder.Fill(matchQuantity, tradePrice)
			counterOrder.Fill(matchQuantity, tradePrice)

			if incomingOrder.Side == Buy {
				ob.TotalAskQuantity -= matchQuantity
			} else {
				ob.TotalBidQuantity -= matchQuantity
			}
			ob.LastPrice = tradePrice
			ob.LastTradeTime = time.Now().UnixMilli()
			ob.TradeCount++

			trade := NewFuturesTrade(incomingOrder, counterOrder, tradePrice, matchQuantity, 0, "")
			trades = append(trades, trade)

			priceLevel.Quantity -= matchQuantity

			if counterOrder.IsDone() {
				priceLevel.Orders = append(priceLevel.Orders[:i], priceLevel.Orders[i+1:]...)
				i--
			}
		}

		if priceLevel.Quantity <= 0 || len(priceLevel.Orders) == 0 {
			delete(book, price)
		}
	}

	return trades
}

// GetDepth 获取订单簿深度
// levels: 返回的档位数量
func (ob *FuturesOrderBook) GetDepth(levels int) (bids, asks []PricePoint) {
	ob.Mu.RLock()
	defer ob.Mu.RUnlock()

	bidPrices := make([]float64, 0, len(ob.Bids))
	for price := range ob.Bids {
		bidPrices = append(bidPrices, price)
	}
	for i := 0; i < len(bidPrices); i++ {
		for j := i + 1; j < len(bidPrices); j++ {
			if bidPrices[i] < bidPrices[j] {
				bidPrices[i], bidPrices[j] = bidPrices[j], bidPrices[i]
			}
		}
	}

	for i := 0; i < len(bidPrices) && i < levels; i++ {
		if priceLevel, ok := ob.Bids[bidPrices[i]]; ok {
			bids = append(bids, PricePoint{Price: priceLevel.Price, Quantity: priceLevel.Quantity})
		}
	}

	askPrices := make([]float64, 0, len(ob.Asks))
	for price := range ob.Asks {
		askPrices = append(askPrices, price)
	}
	for i := 0; i < len(askPrices); i++ {
		for j := i + 1; j < len(askPrices); j++ {
			if askPrices[i] > askPrices[j] {
				askPrices[i], askPrices[j] = askPrices[j], askPrices[i]
			}
		}
	}

	for i := 0; i < len(askPrices) && i < levels; i++ {
		if priceLevel, ok := ob.Asks[askPrices[i]]; ok {
			asks = append(asks, PricePoint{Price: priceLevel.Price, Quantity: priceLevel.Quantity})
		}
	}

	return bids, asks
}

// GetBestBidAsk 获取最优买卖盘价格和数量
func (ob *FuturesOrderBook) GetBestBidAsk() (bestBid, bestAsk float64, bidQty, askQty float64) {
	ob.Mu.RLock()
	defer ob.Mu.RUnlock()

	bestBid = 0
	for price, level := range ob.Bids {
		if price > bestBid {
			bestBid = price
			bidQty = level.Quantity
		}
	}

	bestAsk = math.MaxFloat64
	for price, level := range ob.Asks {
		if price < bestAsk {
			bestAsk = price
			askQty = level.Quantity
		}
	}

	if bestBid == 0 {
		bestBid = 0
		bidQty = 0
	}
	if bestAsk == math.MaxFloat64 {
		bestAsk = 0
		askQty = 0
	}

	return
}

// PricePoint 价格点
// 用于订单簿深度数据结构
type PricePoint struct {
	Price    float64 // 价格
	Quantity float64 // 数量
}

// FuturesEngine 合约撮合引擎
// 核心组件，处理订单提交、撮合匹配、持仓管理
// [Design: 系统架构](../DESIGN_DERIVATIVES.md#21-整体架构图)
type FuturesEngine struct {
	symbol        string              // 交易对
	orderBook     *FuturesOrderBook   // 订单簿
	positions     map[string]*Position // 持仓映射
	tradeCh       chan *FuturesTrade  // 交易事件通道
	orderCh       chan *FuturesOrder  // 订单提交通道
	doneCh        chan struct{}       // 停止信号通道

	processedCount int64 // 已处理订单数
	failedCount    int64 // 失败订单数

	logger zerolog.Logger // 日志
	ctx    context.Context // 上下文
	cancel context.CancelFunc // 取消函数
	wg     sync.WaitGroup    // 等待组
}

// NewFuturesEngine 创建新的合约撮合引擎
func NewFuturesEngine(symbol string, logger zerolog.Logger) *FuturesEngine {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &FuturesEngine{
		symbol:    symbol,
		orderBook: NewFuturesOrderBook(symbol),
		positions: make(map[string]*Position),
		tradeCh:   make(chan *FuturesTrade, 50000),
		orderCh:   make(chan *FuturesOrder, 50000),
		doneCh:    make(chan struct{}),
		logger:    logger.With().Str("symbol", symbol).Logger(),
		ctx:       ctx,
		cancel:    cancel,
	}

	engine.wg.Add(1)
	go engine.run()

	return engine
}

// run 引擎主循环
// 监听订单通道，处理订单提交
func (e *FuturesEngine) run() {
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
// 根据订单类型选择处理方式
func (e *FuturesEngine) processOrder(order *FuturesOrder) {
	atomic.AddInt64(&e.processedCount, 1)

	if order.Type == FuturesMarketOrder {
		e.processMarketOrder(order)
		return
	}

	trades := e.orderBook.Match(order)

	if len(trades) > 0 {
		e.logger.Info().Int("trade_count", len(trades)).Msg("Futures order matched")

		for _, trade := range trades {
			select {
			case e.tradeCh <- trade:
			default:
				e.logger.Warn().Str("trade_id", trade.TradeID).Msg("Trade channel full")
			}
		}

		e.updatePositions(order, trades)
	}

	if !order.IsDone() && order.Status != 3 {
		e.orderBook.AddOrder(order)
	}
}

// processMarketOrder 处理市价订单
// 市价单优先匹配最优对手盘
func (e *FuturesEngine) processMarketOrder(order *FuturesOrder) {
	trades := e.orderBook.Match(order)

	if len(trades) > 0 {
		for _, trade := range trades {
			select {
			case e.tradeCh <- trade:
			default:
				e.logger.Warn().Str("trade_id", trade.TradeID).Msg("Trade channel full")
			}
		}
		e.updatePositions(order, trades)
	} else {
		order.Status = 4
		atomic.AddInt64(&e.failedCount, 1)
		e.logger.Warn().Str("order_id", order.OrderID).Msg("Market order rejected")
	}
}

// updatePositions 更新持仓
// 根据成交结果创建或更新用户持仓
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#32-持仓-position)
func (e *FuturesEngine) updatePositions(order *FuturesOrder, trades []*FuturesTrade) {
	for _, trade := range trades {
		positionKey := order.UserID + ":" + order.Symbol + ":" + strconv.Itoa(int(order.PositionSide))

		if position, ok := e.positions[positionKey]; ok {
			position.mu.Lock()
			position.Quantity += trade.Quantity
			if position.EntryPrice == 0 {
				position.EntryPrice = trade.Price
			} else {
				totalQuantity := position.Quantity
				position.EntryPrice = (position.EntryPrice*(totalQuantity-trade.Quantity) + trade.Price*trade.Quantity) / totalQuantity
			}
			position.UpdateTime = time.Now().UnixMilli()
			position.mu.Unlock()
		} else {
			margin := trade.Price * trade.Quantity / float64(order.Leverage)
			position := NewPosition(order.UserID, order.Symbol, order.PositionSide,
				trade.Quantity, trade.Price, margin, order.Leverage, order.MarginType)
			e.positions[positionKey] = position
		}
	}
}

// SubmitOrder 提交订单到引擎
// 超时时间: 100ms
func (e *FuturesEngine) SubmitOrder(order *FuturesOrder) error {
	select {
	case e.orderCh <- order:
		return nil
	case <-time.After(100 * time.Millisecond):
		return &EngineError{Code: "ORDER_CHANNEL_FULL", Message: "Order channel is full"}
	case <-e.ctx.Done():
		return &EngineError{Code: "ENGINE_STOPPED", Message: "Engine has been stopped"}
	}
}

// GetTrades 获取交易记录
func (e *FuturesEngine) GetTrades() []*FuturesTrade {
	var trades []*FuturesTrade
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
func (e *FuturesEngine) GetOrderBook() *OrderBookSnapshot {
	bids, asks := e.orderBook.GetDepth(20)

	return &OrderBookSnapshot{
		Symbol:    e.symbol,
		Version:   time.Now().UnixNano(),
		Timestamp: time.Now().UnixMilli(),
		Bids:      bids,
		Asks:      asks,
		LastPrice: e.orderBook.LastPrice,
	}
}

// GetDepth 获取订单簿深度
func (e *FuturesEngine) GetDepth(levels int) (bids, asks []PricePoint) {
	return e.orderBook.GetDepth(levels)
}

// GetBestBidAsk 获取最优买卖盘
func (e *FuturesEngine) GetBestBidAsk() (bestBid, bestAsk, bidQty, askQty float64) {
	return e.orderBook.GetBestBidAsk()
}

// GetPositions 获取用户所有持仓
func (e *FuturesEngine) GetPositions(userID string) []*Position {
	var positions []*Position
	for _, position := range e.positions {
		if position.UserID == userID {
			positions = append(positions, position)
		}
	}
	return positions
}

// GetPosition 获取用户特定方向的持仓
func (e *FuturesEngine) GetPosition(userID, symbol string, side PositionSide) *Position {
	positionKey := userID + ":" + symbol + ":" + strconv.Itoa(int(side))
	return e.positions[positionKey]
}

// Stop 停止引擎
func (e *FuturesEngine) Stop() {
	e.cancel()
	e.wg.Wait()
	close(e.tradeCh)
	close(e.orderCh)
	close(e.doneCh)
}

// GetStats 获取引擎统计信息
func (e *FuturesEngine) GetStats() EngineStats {
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

// EngineStats 引擎统计信息结构
type EngineStats struct {
	Symbol         string  // 交易对
	ProcessedCount int64   // 已处理订单数
	FailedCount    int64   // 失败订单数
	TotalBidQty    float64 // 买单总数量
	TotalAskQty    float64 // 卖单总数量
	TradeCount     int64   // 成交次数
	LastPrice      float64 // 最新成交价
	LastTradeTime  int64   // 最新成交时间
}

// OrderBookSnapshot 订单簿快照结构
type OrderBookSnapshot struct {
	Symbol    string       // 交易对
	Version   int64        // 版本号
	Timestamp int64        // 时间戳
	Bids      []PricePoint // 买单深度
	Asks      []PricePoint // 卖单深度
	LastPrice float64      // 最新成交价
}

// GetBestBidAsk 获取快照中的最优买卖盘
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

// EngineError 引擎错误结构
type EngineError struct {
	Code    string // 错误码
	Message string // 错误消息
}

// Error 返回错误字符串
func (e *EngineError) Error() string {
	return e.Code + ": " + e.Message
}
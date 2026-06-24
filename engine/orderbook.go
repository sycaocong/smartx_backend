package engine

import (
	"math"
	"sync"
)

// PriceLevel 价格档位
type PriceLevel struct {
	Price    float64
	Quantity float64
}

// OrderBook 订单簿
type OrderBook struct {
	Symbol string
	Bids   *SkipList // 买单（价格降序）
	Asks   *SkipList // 卖单（价格升序）
	Mu     sync.RWMutex

	// 统计信息
	TotalBidQuantity float64
	TotalAskQuantity float64
	TradeCount       int64
	LastPrice        float64
	LastTradeTime    int64
}

// NewOrderBook 创建订单簿
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol: symbol,
		Bids:   NewSkipList(true),  // 买单按价格降序
		Asks:   NewSkipList(false), // 卖单按价格升序
	}
}

// AddOrder 添加订单
func (ob *OrderBook) AddOrder(order *Order) {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var skiplist *SkipList
	if order.Side == Buy {
		skiplist = ob.Bids
		ob.TotalBidQuantity += order.Quantity
	} else {
		skiplist = ob.Asks
		ob.TotalAskQuantity += order.Quantity
	}

	// 添加到跳表
	node := &OrderNode{Order: order}
	skiplist.Insert(order.Price, order.Priority, node)
}

// RemoveOrder 移除订单
func (ob *OrderBook) RemoveOrder(order *Order) bool {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var skiplist *SkipList
	remainingQty := order.RemainingQuantityUnsafe()
	if order.Side == Buy {
		skiplist = ob.Bids
		ob.TotalBidQuantity -= remainingQty
	} else {
		skiplist = ob.Asks
		ob.TotalAskQuantity -= remainingQty
	}

	// 从跳表中移除
	return skiplist.Delete(order.Price, order.Priority)
}

// Match 撮合
func (ob *OrderBook) Match(incomingOrder *Order) []*Trade {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var trades []*Trade

	if incomingOrder.Side == Buy {
		// 买单匹配卖单
		trades = ob.matchOrders(incomingOrder, ob.Asks, Buy)
	} else {
		// 卖单匹配买单
		trades = ob.matchOrders(incomingOrder, ob.Bids, Sell)
	}

	return trades
}

// matchOrders 执行订单匹配
func (ob *OrderBook) matchOrders(incoming *Order, book *SkipList, oppositeSide OrderSide) []*Trade {
	var trades []*Trade

	// 确定价格条件：买单需要 incoming.Price >= book.Price，卖单需要 incoming.Price <= book.Price
	for book.Size > 0 {
		bestNode := book.Min()
		if bestNode == nil {
			break
		}

		bestPrice := bestNode.Key

		// 检查价格是否匹配
		var priceMatch bool
		if incoming.Side == Buy {
			priceMatch = incoming.Price >= bestPrice
		} else {
			priceMatch = incoming.Price <= bestPrice
		}

		if !priceMatch {
			break
		}

		// 获取对手订单
		orderNode := bestNode.Value.(*OrderNode)
		counterOrder := orderNode.Order

		// 跳过已完成的订单
		if counterOrder.IsDone() {
			book.Delete(bestPrice, bestNode.Score)
			continue
		}

		// 计算成交数量
		remainingIncoming := incoming.RemainingQuantity()
		remainingCounter := counterOrder.RemainingQuantity()
		matchQuantity := math.Min(remainingIncoming, remainingCounter)

		// 使用对手方价格成交（LIMIT订单按盘口价成交）
		tradePrice := counterOrder.Price

		// 成交
		incoming.Fill(matchQuantity, tradePrice)
		counterOrder.Fill(matchQuantity, tradePrice)

		// 更新统计
		if incoming.Side == Buy {
			ob.TotalAskQuantity -= matchQuantity
		} else {
			ob.TotalBidQuantity -= matchQuantity
		}
		ob.LastPrice = tradePrice
		ob.LastTradeTime = incoming.Timestamp
		ob.TradeCount++

		// 创建成交记录
		trade := NewTrade(incoming, counterOrder, tradePrice, matchQuantity, 0, "")
		trades = append(trades, trade)

		// 如果对手订单完成，从订单簿移除
		if counterOrder.IsDone() {
			book.Delete(bestPrice, bestNode.Score)
		}

		// 如果输入订单完成，退出循环
		if incoming.IsDone() {
			break
		}
	}

	return trades
}

// GetDepth 获取深度
func (ob *OrderBook) GetDepth(levels int) (bids, asks []PricePoint) {
	ob.Mu.RLock()
	defer ob.Mu.RUnlock()

	// 获取买单深度
	bidNodes := ob.Bids.Range(nil, nil, levels)
	for _, node := range bidNodes {
		if priceLevel, ok := node.Value.(*PriceLevel); ok {
			bids = append(bids, PricePoint{Price: priceLevel.Price, Quantity: priceLevel.Quantity})
		}
	}

	// 获取卖单深度
	askNodes := ob.Asks.Range(nil, nil, levels)
	for _, node := range askNodes {
		if priceLevel, ok := node.Value.(*PriceLevel); ok {
			asks = append(asks, PricePoint{Price: priceLevel.Price, Quantity: priceLevel.Quantity})
		}
	}

	return bids, asks
}

// GetBestBidAsk 获取最佳买卖价
func (ob *OrderBook) GetBestBidAsk() (bestBid, bestAsk float64, bidQty, askQty float64) {
	ob.Mu.RLock()
	defer ob.Mu.RUnlock()

	if ob.Bids.Size > 0 {
		minBid := ob.Bids.Min()
		if minBid != nil {
			bestBid = minBid.Key
			if priceLevel, ok := minBid.Value.(*PriceLevel); ok {
				bidQty = priceLevel.Quantity
			}
		}
	}

	if ob.Asks.Size > 0 {
		minAsk := ob.Asks.Min()
		if minAsk != nil {
			bestAsk = minAsk.Key
			if priceLevel, ok := minAsk.Value.(*PriceLevel); ok {
				askQty = priceLevel.Quantity
			}
		}
	}

	return bestBid, bestAsk, bidQty, askQty
}

// PricePoint 价格点
type PricePoint struct {
	Price    float64
	Quantity float64
}

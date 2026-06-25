package engine

import (
	"math"
	"sync"
)

// PriceLevel 价格档位（按价格聚合的订单列表）
type PriceLevel struct {
	Price    float64
	Quantity float64
	Orders   []*Order
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

	// 使用价格作为key（同价格只存一个节点），Priority固定为0
	existingNode := skiplist.Search(order.Price, 0)
	if existingNode != nil {
		// 更新现有价格档位
		if priceLevel, ok := existingNode.Value.(*PriceLevel); ok {
			priceLevel.Quantity += order.Quantity
			priceLevel.Orders = append(priceLevel.Orders, order)
		}
	} else {
		// 创建新的价格档位
		priceLevel := &PriceLevel{
			Price:    order.Price,
			Quantity: order.Quantity,
			Orders:   []*Order{order},
		}
		skiplist.Insert(order.Price, 0, priceLevel)
	}
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

	// 检查并更新价格档位
	existingNode := skiplist.Search(order.Price, 0)
	if existingNode != nil {
		if priceLevel, ok := existingNode.Value.(*PriceLevel); ok {
			priceLevel.Quantity -= remainingQty
			// 从订单列表中移除
			for i, o := range priceLevel.Orders {
				if o.OrderID == order.OrderID {
					priceLevel.Orders = append(priceLevel.Orders[:i], priceLevel.Orders[i+1:]...)
					break
				}
			}
			if priceLevel.Quantity <= 0 || len(priceLevel.Orders) == 0 {
				// 删除空的价格档位
				return skiplist.Delete(order.Price, 0)
			}
			return true
		}
	}

	return false
}

// Match 撮合
func (ob *OrderBook) Match(incomingOrder *Order) []*Trade {
	ob.Mu.Lock()
	defer ob.Mu.Unlock()

	var trades []*Trade

	if incomingOrder.Side == Buy {
		// 买单匹配卖单
		trades = ob.matchOrders(incomingOrder, ob.Asks)
	} else {
		// 卖单匹配买单
		trades = ob.matchOrders(incomingOrder, ob.Bids)
	}

	return trades
}

// matchOrders 执行订单匹配
func (ob *OrderBook) matchOrders(incoming *Order, book *SkipList) []*Trade {
	var trades []*Trade

	// 确定价格条件：买单需要 incoming.Price >= book.Price，卖单需要 incoming.Price <= book.Price
	for book.Size > 0 {
		bestNode := book.Min()
		if bestNode == nil {
			break
		}

		bestPrice := bestNode.Key

		// 检查价格是否匹配（市价单跳过价格检查）
		if incoming.Type != MarketOrder {
			var priceMatch bool
			if incoming.Side == Buy {
				priceMatch = incoming.Price >= bestPrice
			} else {
				priceMatch = incoming.Price <= bestPrice
			}

			if !priceMatch {
				break
			}
		}

		// 获取价格档位
		priceLevel := bestNode.Value.(*PriceLevel)
		
		// 遍历该价格档位的所有订单进行撮合
		for i := 0; i < len(priceLevel.Orders); i++ {
			counterOrder := priceLevel.Orders[i]
			
			// 跳过已完成的订单
			if counterOrder.IsDone() {
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

			// 更新价格档位数量
			priceLevel.Quantity -= matchQuantity

			// 如果输入订单完成，退出循环
			if incoming.IsDone() {
				break
			}
		}

		// 如果价格档位的所有订单都完成，删除该价格档位
		allDone := true
		for _, o := range priceLevel.Orders {
			if !o.IsDone() {
				allDone = false
				break
			}
		}
		if allDone || priceLevel.Quantity <= 0 {
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

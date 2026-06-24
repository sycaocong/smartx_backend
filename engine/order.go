package engine

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrderSide 订单方向
type OrderSide int

const (
	Buy OrderSide = iota
	Sell
)

// OrderType 订单类型
type OrderType int

const (
	LimitOrder OrderType = iota
	MarketOrder
	StopLimitOrder
)

// OrderStatus 订单状态
type OrderStatus int

const (
	StatusNew OrderStatus = iota
	StatusPartiallyFilled
	StatusFilled
	StatusCanceled
	StatusRejected
)

// Order 订单结构
type Order struct {
	OrderID        string    // 订单ID
	Symbol         string    // 交易对
	Side           OrderSide // 买卖方向
	Type           OrderType // 订单类型
	Price          float64   // 价格
	Quantity       float64   // 数量
	FilledQuantity float64   // 已成交数量
	AvgFillPrice   float64   // 平均成交价
	Status         OrderStatus
	Timestamp      int64  // 时间戳
	ClientOrderID  string // 客户端订单ID
	Priority       int64  // 优先级（时间戳）

	// 内部字段
	muRW sync.RWMutex
	cond *sync.Cond
	done bool
}

// Lock 实现 sync.Locker 接口
func (o *Order) Lock() {
	o.muRW.Lock()
}

// Unlock 实现 sync.Locker 接口
func (o *Order) Unlock() {
	o.muRW.Unlock()
}

// NewOrder 创建新订单
func NewOrder(symbol string, side OrderSide, orderType OrderType, price, quantity float64, clientOrderID string) *Order {
	order := &Order{
		OrderID:        uuid.New().String(),
		Symbol:         symbol,
		Side:           side,
		Type:           orderType,
		Price:          price,
		Quantity:       quantity,
		FilledQuantity: 0,
		AvgFillPrice:   0,
		Status:         StatusNew,
		Timestamp:      time.Now().UnixMilli(),
		ClientOrderID:  clientOrderID,
		Priority:       time.Now().UnixNano(),
	}
	order.cond = sync.NewCond(order)
	return order
}

// RemainingQuantity 获取剩余数量
func (o *Order) RemainingQuantity() float64 {
	o.muRW.RLock()
	defer o.muRW.RUnlock()
	return o.Quantity - o.FilledQuantity
}

// RemainingQuantityUnsafe 在已持有锁的情况下获取剩余数量
func (o *Order) RemainingQuantityUnsafe() float64 {
	return o.Quantity - o.FilledQuantity
}

// Fill 成交
func (o *Order) Fill(quantity, price float64) {
	o.muRW.Lock()
	defer o.muRW.Unlock()

	// 更新已成交数量
	newFilled := o.FilledQuantity + quantity

	// 如果是首笔成交，直接使用该价格
	if o.FilledQuantity == 0 {
		o.AvgFillPrice = price
	} else {
		// 计算新的加权平均价
		totalValue := o.AvgFillPrice*o.FilledQuantity + price*quantity
		o.AvgFillPrice = totalValue / newFilled
	}

	o.FilledQuantity = newFilled

	// 更新状态
	if o.FilledQuantity >= o.Quantity {
		o.Status = StatusFilled
		o.done = true
		o.cond.Signal()
	} else {
		o.Status = StatusPartiallyFilled
	}
}

// Cancel 取消订单
func (o *Order) Cancel() {
	o.muRW.Lock()
	defer o.muRW.Unlock()
	if !o.done {
		o.Status = StatusCanceled
		o.done = true
		o.cond.Signal()
	}
}

// Wait 等待订单完成
func (o *Order) Wait(timeout time.Duration) bool {
	o.muRW.Lock()
	defer o.muRW.Unlock()

	if o.done {
		return true
	}

	timeoutCh := time.After(timeout)
	o.cond.Wait()
	select {
	case <-timeoutCh:
		return o.done
	default:
		return o.done
	}
}

// IsDone 检查订单是否完成
func (o *Order) IsDone() bool {
	o.muRW.RLock()
	defer o.muRW.RUnlock()
	return o.done
}

// ToProto 转换为 protobuf 格式
func (o *Order) ToProto() map[string]interface{} {
	return map[string]interface{}{
		"order_id":        o.OrderID,
		"symbol":          o.Symbol,
		"side":            o.Side,
		"type":            o.Type,
		"price":           o.Price,
		"quantity":        o.Quantity,
		"filled_quantity": o.FilledQuantity,
		"avg_fill_price":  o.AvgFillPrice,
		"status":          o.Status,
		"timestamp":       o.Timestamp,
		"client_order_id": o.ClientOrderID,
	}
}

// Trade 成交记录
type Trade struct {
	TradeID        string    // 成交ID
	Symbol         string    // 交易对
	OrderID        string    // 订单ID
	CounterOrderID string    // 对手订单ID
	Side           OrderSide // 方向
	Price          float64   // 成交价
	Quantity       float64   // 成交数量
	Fee            float64   // 手续费
	FeeCurrency    string    // 手续费货币
	Timestamp      int64     // 时间戳
}

// NewTrade 创建成交记录
func NewTrade(order, counterOrder *Order, price, quantity float64, fee float64, feeCurrency string) *Trade {
	return &Trade{
		TradeID:        uuid.New().String(),
		Symbol:         order.Symbol,
		OrderID:        order.OrderID,
		CounterOrderID: counterOrder.OrderID,
		Side:           order.Side,
		Price:          price,
		Quantity:       quantity,
		Fee:            fee,
		FeeCurrency:    feeCurrency,
		Timestamp:      time.Now().UnixMilli(),
	}
}

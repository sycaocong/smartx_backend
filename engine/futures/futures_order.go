package futures

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// OrderSide 订单方向枚举
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#31-合约订单-futuresorder)
type OrderSide int

const (
	Buy  OrderSide = iota // 买单
	Sell                  // 卖单
)

// FuturesOrderType 合约订单类型枚举
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#31-合约订单-futuresorder)
type FuturesOrderType int

const (
	FuturesLimitOrder   FuturesOrderType = iota // 限价单
	FuturesMarketOrder                          // 市价单
	FuturesStopOrder                            // 止损单
	FuturesIcebergOrder                         // 冰山订单
)

// MarginType 保证金类型枚举
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#31-合约订单-futuresorder)
type MarginType int

const (
	CrossMargin    MarginType = iota // 全仓保证金
	IsolatedMargin                   // 逐仓保证金
)

// PositionSide 持仓方向枚举
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#32-持仓-position)
type PositionSide int

const (
	Long  PositionSide = iota // 多头持仓
	Short                     // 空头持仓
)

// PositionStatus 持仓状态枚举
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#32-持仓-position)
type PositionStatus int

const (
	PositionOpen      PositionStatus = iota // 持仓中
	PositionClosed                          // 已平仓
	PositionLiquidated                      // 已强平
)

// FuturesOrder 合约订单结构
// 包含订单的所有属性，支持杠杆交易和多种订单类型
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#31-合约订单-futuresorder)
type FuturesOrder struct {
	OrderID          string           `json:"order_id"`          // 订单唯一ID
	Symbol           string           `json:"symbol"`            // 交易对
	Side             OrderSide        `json:"side"`              // 买卖方向
	Type             FuturesOrderType `json:"type"`              // 订单类型
	Price            float64          `json:"price"`             // 委托价格
	Quantity         float64          `json:"quantity"`          // 委托数量
	Leverage         int              `json:"leverage"`          // 杠杆倍数(1-100)
	MarginType       MarginType       `json:"margin_type"`       // 保证金类型
	PositionSide     PositionSide     `json:"position_side"`     // 持仓方向(开仓时指定)
	EntryPrice       float64          `json:"entry_price"`       // 实际开仓价格
	LiquidationPrice float64          `json:"liquidation_price"` // 强平价格

	FilledQuantity float64 `json:"filled_quantity"` // 已成交数量
	AvgFillPrice   float64 `json:"avg_fill_price"`   // 平均成交价格
	Status         int     `json:"status"`           // 订单状态(0:待成交,1:部分成交,2:已成交,3:已取消,4:已拒绝)
	Timestamp      int64   `json:"timestamp"`        // 创建时间戳
	ClientOrderID  string  `json:"client_order_id"`  // 客户端订单ID
	UserID         string  `json:"user_id"`          // 用户ID

	muRW sync.RWMutex // 读写锁，保证并发安全
	done bool         // 订单是否已完成(成交/取消/拒绝)
}

// NewFuturesOrder 创建新的合约订单
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#31-合约订单-futuresorder)
func NewFuturesOrder(symbol string, side OrderSide, orderType FuturesOrderType,
	price, quantity float64, leverage int, marginType MarginType,
	positionSide PositionSide, userID, clientOrderID string) *FuturesOrder {

	return &FuturesOrder{
		OrderID:          uuid.New().String(),
		Symbol:           symbol,
		Side:             side,
		Type:             orderType,
		Price:            price,
		Quantity:         quantity,
		Leverage:         leverage,
		MarginType:       marginType,
		PositionSide:     positionSide,
		FilledQuantity:   0,
		AvgFillPrice:     0,
		Status:           0,
		Timestamp:        time.Now().UnixMilli(),
		ClientOrderID:    clientOrderID,
		UserID:           userID,
	}
}

// RemainingQuantity 返回订单剩余未成交数量
func (o *FuturesOrder) RemainingQuantity() float64 {
	o.muRW.RLock()
	defer o.muRW.RUnlock()
	return o.Quantity - o.FilledQuantity
}

// Fill 成交订单的一部分
// quantity: 成交数量
// price: 成交价格
// [Design: 撮合算法](../DESIGN_DERIVATIVES.md#45-撮合算法)
func (o *FuturesOrder) Fill(quantity, price float64) {
	o.muRW.Lock()
	defer o.muRW.Unlock()

	newFilled := o.FilledQuantity + quantity

	if o.FilledQuantity == 0 {
		o.AvgFillPrice = price
	} else {
		totalValue := o.AvgFillPrice*o.FilledQuantity + price*quantity
		o.AvgFillPrice = totalValue / newFilled
	}

	o.FilledQuantity = newFilled

	if o.FilledQuantity >= o.Quantity {
		o.Status = 2
		o.done = true
	} else {
		o.Status = 1
	}
}

// Cancel 取消订单
func (o *FuturesOrder) Cancel() {
	o.muRW.Lock()
	defer o.muRW.Unlock()
	if !o.done {
		o.Status = 3
		o.done = true
	}
}

// IsDone 订单是否已完成(成交/取消/拒绝)
func (o *FuturesOrder) IsDone() bool {
	o.muRW.RLock()
	defer o.muRW.RUnlock()
	return o.done
}

// ToProto 将订单转换为可序列化的协议格式
func (o *FuturesOrder) ToProto() map[string]interface{} {
	return map[string]interface{}{
		"order_id":          o.OrderID,
		"symbol":            o.Symbol,
		"side":              o.Side,
		"type":              o.Type,
		"price":             o.Price,
		"quantity":          o.Quantity,
		"leverage":          o.Leverage,
		"margin_type":       o.MarginType,
		"position_side":     o.PositionSide,
		"filled_quantity":   o.FilledQuantity,
		"avg_fill_price":    o.AvgFillPrice,
		"status":            o.Status,
		"timestamp":         o.Timestamp,
		"client_order_id":   o.ClientOrderID,
		"user_id":           o.UserID,
		"liquidation_price": o.LiquidationPrice,
	}
}

// Position 持仓结构
// 记录用户在某交易对上的持仓信息，包括盈亏、保证金等
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#32-持仓-position)
type Position struct {
	PositionID       string         `json:"position_id"`       // 持仓唯一ID
	Symbol           string         `json:"symbol"`            // 交易对
	UserID           string         `json:"user_id"`           // 用户ID
	Side             PositionSide   `json:"side"`              // 持仓方向(Long/Short)
	Quantity         float64        `json:"quantity"`          // 持仓数量
	EntryPrice       float64        `json:"entry_price"`       // 开仓价格
	MarkPrice        float64        `json:"mark_price"`        // 当前标记价格
	LiquidationPrice float64        `json:"liquidation_price"` // 强平价格
	Margin           float64        `json:"margin"`            // 保证金金额
	Leverage         int            `json:"leverage"`          // 杠杆倍数
	MarginType       MarginType     `json:"margin_type"`       // 保证金类型

	UnrealizedPNL float64      `json:"unrealized_pnl"` // 未实现盈亏
	RealizedPNL   float64      `json:"realized_pnl"`   // 已实现盈亏
	FundingFee    float64      `json:"funding_fee"`    // 累计资金费用

	Status   PositionStatus `json:"status"`     // 持仓状态
	OpenTime int64          `json:"open_time"`  // 开仓时间
	UpdateTime int64        `json:"update_time"` // 更新时间

	mu sync.RWMutex // 读写锁，保证并发安全
}

// NewPosition 创建新的持仓
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#32-持仓-position)
func NewPosition(userID, symbol string, side PositionSide, quantity, entryPrice, margin float64,
	leverage int, marginType MarginType) *Position {

	return &Position{
		PositionID:       uuid.New().String(),
		UserID:           userID,
		Symbol:           symbol,
		Side:             side,
		Quantity:         quantity,
		EntryPrice:       entryPrice,
		MarkPrice:        entryPrice,
		Margin:           margin,
		Leverage:         leverage,
		MarginType:       marginType,
		UnrealizedPNL:    0,
		RealizedPNL:      0,
		FundingFee:       0,
		Status:           PositionOpen,
		OpenTime:         time.Now().UnixMilli(),
		UpdateTime:       time.Now().UnixMilli(),
	}
}

// UpdateMarkPrice 更新标记价格
// [Design: 标记价格计算](../DESIGN_DERIVATIVES.md#41-标记价格计算-mark-price)
func (p *Position) UpdateMarkPrice(markPrice float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.MarkPrice = markPrice
	p.UpdateTime = time.Now().UnixMilli()
}

// UpdateUnrealizedPNL 更新未实现盈亏
// 计算公式: Long: Quantity * (MarkPrice - EntryPrice)
//           Short: Quantity * (EntryPrice - MarkPrice)
// [Design: 保证金计算](../DESIGN_DERIVATIVES.md#42-保证金计算)
func (p *Position) UpdateUnrealizedPNL() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Side == Long {
		p.UnrealizedPNL = p.Quantity * (p.MarkPrice - p.EntryPrice)
	} else {
		p.UnrealizedPNL = p.Quantity * (p.EntryPrice - p.MarkPrice)
	}
}

// AddFundingFee 添加资金费用
// 资金费用每8小时结算一次，影响保证金余额
// [Design: 资金费率计算](../DESIGN_DERIVATIVES.md#44-资金费率计算)
func (p *Position) AddFundingFee(fee float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FundingFee += fee
	p.Margin -= fee
	p.UpdateTime = time.Now().UnixMilli()
}

// Close 平仓
// closePrice: 平仓价格
func (p *Position) Close(closePrice float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Side == Long {
		p.RealizedPNL += p.Quantity * (closePrice - p.EntryPrice)
	} else {
		p.RealizedPNL += p.Quantity * (p.EntryPrice - closePrice)
	}

	p.Status = PositionClosed
	p.Quantity = 0
	p.UpdateTime = time.Now().UnixMilli()
}

// Liquidate 强制平仓
// liquidationPrice: 强平执行价格
// [Design: 强制平仓](../DESIGN_DERIVATIVES.md#63-强制平仓)
func (p *Position) Liquidate(liquidationPrice float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Side == Long {
		p.RealizedPNL += p.Quantity * (liquidationPrice - p.EntryPrice)
	} else {
		p.RealizedPNL += p.Quantity * (p.EntryPrice - liquidationPrice)
	}

	p.Status = PositionLiquidated
	p.Quantity = 0
	p.UpdateTime = time.Now().UnixMilli()
}

// ToProto 将持仓转换为可序列化的协议格式
func (p *Position) ToProto() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"position_id":       p.PositionID,
		"symbol":            p.Symbol,
		"user_id":           p.UserID,
		"side":              p.Side,
		"quantity":          p.Quantity,
		"entry_price":       p.EntryPrice,
		"mark_price":        p.MarkPrice,
		"liquidation_price": p.LiquidationPrice,
		"margin":            p.Margin,
		"leverage":          p.Leverage,
		"margin_type":       p.MarginType,
		"unrealized_pnl":    p.UnrealizedPNL,
		"realized_pnl":      p.RealizedPNL,
		"funding_fee":       p.FundingFee,
		"status":            p.Status,
		"open_time":         p.OpenTime,
		"update_time":       p.UpdateTime,
	}
}

// FuturesTrade 合约交易记录结构
// 记录每笔成交的详细信息
// [Design: 核心数据结构](../DESIGN_DERIVATIVES.md#33-交易记录-futurestrade)
type FuturesTrade struct {
	TradeID        string        `json:"trade_id"`         // 交易唯一ID
	Symbol         string        `json:"symbol"`           // 交易对
	OrderID        string        `json:"order_id"`         // 订单ID
	CounterOrderID string        `json:"counter_order_id"` // 对手订单ID
	Side           OrderSide     `json:"side"`             // 方向
	Price          float64       `json:"price"`            // 成交价格
	Quantity       float64       `json:"quantity"`         // 成交数量
	Fee            float64       `json:"fee"`              // 手续费
	FeeCurrency    string        `json:"fee_currency"`     // 手续费币种
	Timestamp      int64         `json:"timestamp"`        // 成交时间
	PositionSide   PositionSide  `json:"position_side"`    // 持仓方向
	UserID         string        `json:"user_id"`          // 用户ID
}

// NewFuturesTrade 创建新的交易记录
func NewFuturesTrade(order, counterOrder *FuturesOrder, price, quantity float64,
	fee float64, feeCurrency string) *FuturesTrade {

	return &FuturesTrade{
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
		PositionSide:   order.PositionSide,
		UserID:         order.UserID,
	}
}
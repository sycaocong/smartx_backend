package risk

import (
	"strconv"
	"sync"

	"github.com/smartx/matching-engine/engine/futures"
)

const (
	MaintenanceMarginRate = 0.005
	InitialMarginRate     = 0.01
	MaxLeverage           = 100
)

// MarginEngine 保证金引擎
// 管理用户持仓的保证金计算和风险管理
// [Design: 风控引擎](../DESIGN_DERIVATIVES.md#43-风控引擎)
type MarginEngine struct {
	positions map[string]*futures.Position // 持仓映射
	mu        sync.RWMutex                // 读写锁
}

// NewMarginEngine 创建新的保证金引擎
func NewMarginEngine() *MarginEngine {
	return &MarginEngine{
		positions: make(map[string]*futures.Position),
	}
}

// AddPosition 添加持仓
func (me *MarginEngine) AddPosition(position *futures.Position) {
	me.mu.Lock()
	defer me.mu.Unlock()
	key := position.UserID + ":" + position.Symbol + ":" + strconv.Itoa(int(position.Side))
	me.positions[key] = position
}

// RemovePosition 移除持仓
func (me *MarginEngine) RemovePosition(userID, symbol string, side futures.PositionSide) {
	me.mu.Lock()
	defer me.mu.Unlock()
	key := userID + ":" + symbol + ":" + strconv.Itoa(int(side))
	delete(me.positions, key)
}

// GetPosition 获取特定持仓
func (me *MarginEngine) GetPosition(userID, symbol string, side futures.PositionSide) *futures.Position {
	me.mu.RLock()
	defer me.mu.RUnlock()
	key := userID + ":" + symbol + ":" + strconv.Itoa(int(side))
	return me.positions[key]
}

// GetPositions 获取用户所有持仓
// userID为空时返回所有持仓
func (me *MarginEngine) GetPositions(userID string) []*futures.Position {
	me.mu.RLock()
	defer me.mu.RUnlock()
	var positions []*futures.Position
	for _, pos := range me.positions {
		if userID == "" || pos.UserID == userID {
			positions = append(positions, pos)
		}
	}
	return positions
}

// CalculateInitialMargin 计算初始保证金
// 公式: 开仓价格 * 数量 / 杠杆
// [Design: 风控引擎](../DESIGN_DERIVATIVES.md#431-保证金计算)
func CalculateInitialMargin(entryPrice, quantity float64, leverage int) float64 {
	return entryPrice * quantity / float64(leverage)
}

// CalculateMaintenanceMargin 计算维持保证金
// 公式: 开仓价格 * 数量 * 维持保证金率(0.5%)
// [Design: 风控引擎](../DESIGN_DERIVATIVES.md#431-保证金计算)
func CalculateMaintenanceMargin(entryPrice, quantity float64) float64 {
	return entryPrice * quantity * MaintenanceMarginRate
}

// CalculateMarginRate 计算保证金率
// 公式: (保证金 + 未实现盈亏) / 所需保证金
// [Design: 风控引擎](../DESIGN_DERIVATIVES.md#431-保证金计算)
func CalculateMarginRate(position *futures.Position, markPrice float64) float64 {
	unrealizedPNL := calculateUnrealizedPNL(position, markPrice)
	equity := position.Margin + unrealizedPNL

	marginRequired := position.MarkPrice * position.Quantity / float64(position.Leverage)

	if marginRequired == 0 {
		return 1.0
	}

	return equity / marginRequired
}

// calculateUnrealizedPNL 计算未实现盈亏
// 多头: 数量 * (标记价格 - 开仓价格)
// 空头: 数量 * (开仓价格 - 标记价格)
func calculateUnrealizedPNL(position *futures.Position, markPrice float64) float64 {
	if position.Side == futures.Long {
		return position.Quantity * (markPrice - position.EntryPrice)
	}
	return position.Quantity * (position.EntryPrice - markPrice)
}

// CalculateLiquidationPrice 计算强平价格
// 多头: 开仓价格 * (1 - 1/杠杆 + 维持保证金率)
// 空头: 开仓价格 * (1 + 1/杠杆 - 维持保证金率)
// [Design: 强平算法](../DESIGN_DERIVATIVES.md#44-强平算法)
func CalculateLiquidationPrice(position *futures.Position) float64 {
	leverage := float64(position.Leverage)

	if position.Side == futures.Long {
		return position.EntryPrice * (1 - 1/leverage + MaintenanceMarginRate)
	}

	return position.EntryPrice * (1 + 1/leverage - MaintenanceMarginRate)
}

// CheckMarginCall 检查是否触发追加保证金通知
// 当保证金率低于维持保证金率时触发
func CheckMarginCall(position *futures.Position, markPrice float64) bool {
	marginRate := CalculateMarginRate(position, markPrice)
	return marginRate < MaintenanceMarginRate
}

// CheckLiquidation 检查是否触发强平
// 多头: 标记价格 <= 强平价格
// 空头: 标记价格 >= 强平价格
// [Design: 强平算法](../DESIGN_DERIVATIVES.md#44-强平算法)
func CheckLiquidation(position *futures.Position, markPrice float64) bool {
	if position.Side == futures.Long {
		return markPrice <= position.LiquidationPrice
	}
	return markPrice >= position.LiquidationPrice
}
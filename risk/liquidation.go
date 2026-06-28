package risk

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/smartx/matching-engine/engine/futures"
)

// LiquidationEngine 强平引擎
// 监控持仓风险，触发保证金通知和强平操作
// [Design: 强平算法](../DESIGN_DERIVATIVES.md#44-强平算法)
type LiquidationEngine struct {
	marginEngine *MarginEngine // 保证金引擎
	logger       zerolog.Logger // 日志
	mu           sync.RWMutex   // 读写锁
}

// NewLiquidationEngine 创建新的强平引擎
func NewLiquidationEngine(marginEngine *MarginEngine, logger zerolog.Logger) *LiquidationEngine {
	return &LiquidationEngine{
		marginEngine: marginEngine,
		logger:       logger.With().Str("module", "liquidation").Logger(),
	}
}

// CheckAndLiquidate 检查并执行强平
// 返回强平结果列表
func (le *LiquidationEngine) CheckAndLiquidate(userID, symbol string, markPrice float64) []*LiquidationResult {
	le.mu.Lock()
	defer le.mu.Unlock()

	var results []*LiquidationResult
	positions := le.marginEngine.GetPositions(userID)

	for _, position := range positions {
		if position.Symbol != symbol {
			continue
		}

		if CheckLiquidation(position, markPrice) {
			result := le.liquidatePosition(position, markPrice)
			results = append(results, result)
		} else if CheckMarginCall(position, markPrice) {
			le.logger.Warn().
				Str("user_id", userID).
				Str("symbol", symbol).
				Str("side", func() string {
					if position.Side == futures.Long {
						return "LONG"
					}
					return "SHORT"
				}()).
				Float64("mark_price", markPrice).
				Float64("liquidation_price", position.LiquidationPrice).
				Msg("Margin call warning")
		}
	}

	return results
}

// liquidatePosition 执行强平操作
// 调用持仓的Liquidate方法，并从保证金引擎移除持仓
func (le *LiquidationEngine) liquidatePosition(position *futures.Position, liquidationPrice float64) *LiquidationResult {
	position.Liquidate(liquidationPrice)

	le.logger.Info().
		Str("position_id", position.PositionID).
		Str("user_id", position.UserID).
		Str("symbol", position.Symbol).
		Str("side", func() string {
			if position.Side == futures.Long {
				return "LONG"
			}
			return "SHORT"
		}()).
		Float64("liquidation_price", liquidationPrice).
		Float64("entry_price", position.EntryPrice).
		Float64("quantity", position.Quantity).
		Msg("Position liquidated")

	le.marginEngine.RemovePosition(position.UserID, position.Symbol, position.Side)

	return &LiquidationResult{
		PositionID:       position.PositionID,
		UserID:           position.UserID,
		Symbol:           position.Symbol,
		Side:             int(position.Side),
		LiquidationPrice: liquidationPrice,
		EntryPrice:       position.EntryPrice,
		Quantity:         position.Quantity,
		PNL:              position.RealizedPNL,
	}
}

// LiquidationResult 强平结果结构
type LiquidationResult struct {
	PositionID       string  `json:"position_id"`       // 持仓ID
	UserID           string  `json:"user_id"`           // 用户ID
	Symbol           string  `json:"symbol"`           // 交易对
	Side             int     `json:"side"`             // 方向(0=多,1=空)
	LiquidationPrice float64 `json:"liquidation_price"` // 强平价格
	EntryPrice       float64 `json:"entry_price"`       // 开仓价格
	Quantity         float64 `json:"quantity"`         // 数量
	PNL              float64 `json:"pnl"`              // 盈亏
}

// UpdateAllLiquidationPrices 更新所有持仓的强平价格
func (le *LiquidationEngine) UpdateAllLiquidationPrices(symbol string, markPrice float64) {
	le.mu.RLock()
	defer le.mu.RUnlock()

	positions := le.marginEngine.GetPositions("")
	for _, position := range positions {
		if position.Symbol == symbol {
			position.LiquidationPrice = CalculateLiquidationPrice(position)
			position.UpdateMarkPrice(markPrice)
			position.UpdateUnrealizedPNL()
		}
	}
}
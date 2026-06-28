package risk

import (
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	FundingInterval        = 8 * time.Hour
	FundingRateLimit       = 0.0075
	MinFundingRateInterval = 1 * time.Minute
)

// FundingRateEngine 资金费率引擎
// 计算资金费率并在每个资金结算周期进行资金划转
// [Design: 资金费率算法](../DESIGN_DERIVATIVES.md#42-资金费率算法)
type FundingRateEngine struct {
	symbol          string           // 交易对
	markPriceEngine *MarkPriceEngine // 标记价格引擎
	marginEngine    *MarginEngine    // 保证金引擎
	fundingRate     float64          // 当前资金费率
	lastFundingTime time.Time        // 上次资金结算时间
	nextFundingTime time.Time        // 下次资金结算时间
	logger          zerolog.Logger   // 日志
	mu              sync.RWMutex     // 读写锁
}

// NewFundingRateEngine 创建新的资金费率引擎
func NewFundingRateEngine(symbol string, markPriceEngine *MarkPriceEngine,
	marginEngine *MarginEngine, logger zerolog.Logger) *FundingRateEngine {

	now := time.Now()
	return &FundingRateEngine{
		symbol:          symbol,
		markPriceEngine: markPriceEngine,
		marginEngine:    marginEngine,
		fundingRate:     0,
		lastFundingTime: now,
		nextFundingTime: now.Add(FundingInterval),
		logger:          logger.With().Str("symbol", symbol).Str("module", "funding_rate").Logger(),
	}
}

// CalculateFundingRate 计算资金费率
// 公式: 资金费率 = clamp((指数价格 - 标记价格) / 指数价格, -0.75%, 0.75%)
// [Design: 资金费率算法](../DESIGN_DERIVATIVES.md#421-资金费率计算)
func (fre *FundingRateEngine) CalculateFundingRate() float64 {
	fre.mu.Lock()
	defer fre.mu.Unlock()

	indexPrice := fre.markPriceEngine.GetIndexPrice()
	markPrice := fre.markPriceEngine.GetMarkPrice()

	if indexPrice == 0 {
		return fre.fundingRate
	}

	baseRate := (indexPrice - markPrice) / indexPrice

	fundingRate := clamp(baseRate, -FundingRateLimit, FundingRateLimit)
	fre.fundingRate = fundingRate

	return fundingRate
}

// clamp 将值限制在min和max之间
func clamp(value, min, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

// SettleFunding 执行资金结算
// 资金费用 = 持仓数量 * 标记价格 * 资金费率
// 多头支付资金费用给空头(当资金费率为正时)
// [Design: 资金费率算法](../DESIGN_DERIVATIVES.md#422-资金结算)
func (fre *FundingRateEngine) SettleFunding() []*FundingSettlement {
	fre.mu.Lock()
	defer fre.mu.Unlock()

	var settlements []*FundingSettlement
	positions := fre.marginEngine.GetPositions("")

	for _, position := range positions {
		if position.Symbol != fre.symbol {
			continue
		}

		fundingFee := position.Quantity * position.MarkPrice * fre.fundingRate

		if position.Side == 0 {
			fundingFee = -fundingFee
		}

		position.AddFundingFee(fundingFee)

		settlements = append(settlements, &FundingSettlement{
			PositionID: position.PositionID,
			UserID:     position.UserID,
			Symbol:     position.Symbol,
			Side:       int(position.Side),
			Quantity:   position.Quantity,
			MarkPrice:  position.MarkPrice,
			FundingRate: fre.fundingRate,
			FundingFee:  fundingFee,
		})

		fre.logger.Info().
			Str("user_id", position.UserID).
			Str("position_id", position.PositionID).
			Float64("funding_fee", fundingFee).
			Float64("funding_rate", fre.fundingRate).
			Msg("Funding fee settled")
	}

	fre.lastFundingTime = time.Now()
	fre.nextFundingTime = fre.lastFundingTime.Add(FundingInterval)

	return settlements
}

// StartFundingCycle 启动资金结算周期
// 每8小时执行一次资金费率计算和结算
func (fre *FundingRateEngine) StartFundingCycle() {
	go func() {
		for {
			now := time.Now()
			duration := fre.nextFundingTime.Sub(now)

			if duration <= 0 {
				fre.CalculateFundingRate()
				fre.SettleFunding()
				continue
			}

			time.Sleep(duration)
		}
	}()
}

// GetFundingRate 获取当前资金费率
func (fre *FundingRateEngine) GetFundingRate() float64 {
	fre.mu.RLock()
	defer fre.mu.RUnlock()
	return fre.fundingRate
}

// GetNextFundingTime 获取下次资金结算时间
func (fre *FundingRateEngine) GetNextFundingTime() time.Time {
	fre.mu.RLock()
	defer fre.mu.RUnlock()
	return fre.nextFundingTime
}

// GetLastFundingTime 获取上次资金结算时间
func (fre *FundingRateEngine) GetLastFundingTime() time.Time {
	fre.mu.RLock()
	defer fre.mu.RUnlock()
	return fre.lastFundingTime
}

// GetSymbol 获取交易对
func (fre *FundingRateEngine) GetSymbol() string {
	return fre.symbol
}

// GetMarkPriceEngine 获取标记价格引擎
func (fre *FundingRateEngine) GetMarkPriceEngine() *MarkPriceEngine {
	return fre.markPriceEngine
}

// FundingSettlement 资金结算结果结构
type FundingSettlement struct {
	PositionID  string  `json:"position_id"`  // 持仓ID
	UserID      string  `json:"user_id"`      // 用户ID
	Symbol      string  `json:"symbol"`       // 交易对
	Side        int     `json:"side"`         // 方向(0=多,1=空)
	Quantity    float64 `json:"quantity"`     // 数量
	MarkPrice   float64 `json:"mark_price"`   // 标记价格
	FundingRate float64 `json:"funding_rate"` // 资金费率
	FundingFee  float64 `json:"funding_fee"`  // 资金费用
}

// FundingRateData 资金费率数据结构
type FundingRateData struct {
	Symbol           string  `json:"symbol"`            // 交易对
	FundingRate      float64 `json:"funding_rate"`      // 资金费率
	NextFundingTime  int64   `json:"next_funding_time"` // 下次资金结算时间
	LastFundingTime  int64   `json:"last_funding_time"` // 上次资金结算时间
	MarkPrice        float64 `json:"mark_price"`        // 标记价格
	IndexPrice       float64 `json:"index_price"`       // 指数价格
	Premium          float64 `json:"premium"`           // 溢价
}
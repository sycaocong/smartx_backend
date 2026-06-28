package risk

import (
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	PriceDeviationThreshold = 0.05
	MaxPriceDeviation       = 0.05
)

// MarkPriceEngine 标记价格引擎
// 计算标记价格防止价格操纵，标记价格 = 指数价格 + 资金费率溢价
// [Design: 标记价格算法](../DESIGN_DERIVATIVES.md#41-标记价格算法)
type MarkPriceEngine struct {
	symbol         string             // 交易对
	indexPrice     float64            // 指数价格
	markPrice      float64            // 标记价格
	lastTradePrice float64            // 最新成交价
	premium        float64            // 溢价
	exchangePrices map[string]float64 // 各交易所价格
	mu             sync.RWMutex       // 读写锁
}

// NewMarkPriceEngine 创建新的标记价格引擎
func NewMarkPriceEngine(symbol string, logger zerolog.Logger) *MarkPriceEngine {
	return &MarkPriceEngine{
		symbol:         symbol,
		exchangePrices: make(map[string]float64),
	}
}

// SetExchangePrice 设置交易所价格
func (mpe *MarkPriceEngine) SetExchangePrice(exchange string, price float64) {
	mpe.mu.Lock()
	defer mpe.mu.Unlock()
	mpe.exchangePrices[exchange] = price
}

// calculateIndexPriceUnsafe 计算指数价格(不加锁)
// 使用等权重平均各交易所价格
func (mpe *MarkPriceEngine) calculateIndexPriceUnsafe() float64 {
	var totalPrice float64
	var totalWeight float64

	for _, price := range mpe.exchangePrices {
		totalPrice += price
		totalWeight += 1.0
	}

	if totalWeight == 0 {
		return mpe.indexPrice
	}

	return totalPrice / totalWeight
}

// CalculateIndexPrice 计算指数价格
// [Design: 标记价格算法](../DESIGN_DERIVATIVES.md#411-指数价格计算)
func (mpe *MarkPriceEngine) CalculateIndexPrice() float64 {
	mpe.mu.RLock()
	defer mpe.mu.RUnlock()
	return mpe.calculateIndexPriceUnsafe()
}

// CalculateMarkPrice 计算标记价格
// 公式: 标记价格 = 指数价格 * (1 + 资金费率 * 时间因子)
// 时间因子 = 距离下一次资金结算时间 / 8小时
// [Design: 标记价格算法](../DESIGN_DERIVATIVES.md#412-标记价格计算)
func (mpe *MarkPriceEngine) CalculateMarkPrice(fundingRate float64, timeToFunding time.Duration) float64 {
	mpe.mu.Lock()
	defer mpe.mu.Unlock()

	indexPrice := mpe.calculateIndexPriceUnsafe()
	mpe.indexPrice = indexPrice

	timeFactor := timeToFunding.Hours() / 8.0
	premium := fundingRate * timeFactor

	mpe.premium = premium

	markPrice := indexPrice * (1 + premium)

	if mpe.lastTradePrice > 0 {
		deviation := math.Abs(markPrice - mpe.lastTradePrice) / mpe.lastTradePrice
		if deviation > PriceDeviationThreshold {
			limit := mpe.lastTradePrice * (1 + MaxPriceDeviation)
			if markPrice > limit {
				markPrice = limit
			} else if markPrice < mpe.lastTradePrice*(1-MaxPriceDeviation) {
				markPrice = mpe.lastTradePrice * (1 - MaxPriceDeviation)
			}
		}
	}

	mpe.markPrice = markPrice
	return markPrice
}

// UpdateLastTradePrice 更新最新成交价
func (mpe *MarkPriceEngine) UpdateLastTradePrice(price float64) {
	mpe.mu.Lock()
	defer mpe.mu.Unlock()
	mpe.lastTradePrice = price
}

// GetMarkPrice 获取标记价格
func (mpe *MarkPriceEngine) GetMarkPrice() float64 {
	mpe.mu.RLock()
	defer mpe.mu.RUnlock()
	return mpe.markPrice
}

// GetIndexPrice 获取指数价格
func (mpe *MarkPriceEngine) GetIndexPrice() float64 {
	mpe.mu.RLock()
	defer mpe.mu.RUnlock()
	return mpe.indexPrice
}

// GetPremium 获取溢价
func (mpe *MarkPriceEngine) GetPremium() float64 {
	mpe.mu.RLock()
	defer mpe.mu.RUnlock()
	return mpe.premium
}

// GetSymbol 获取交易对
func (mpe *MarkPriceEngine) GetSymbol() string {
	return mpe.symbol
}

// MarkPriceData 标记价格数据结构
type MarkPriceData struct {
	Symbol     string  `json:"symbol"`      // 交易对
	MarkPrice  float64 `json:"mark_price"`  // 标记价格
	IndexPrice float64 `json:"index_price"` // 指数价格
	Premium    float64 `json:"premium"`     // 溢价
	Timestamp  int64   `json:"timestamp"`   // 时间戳
}
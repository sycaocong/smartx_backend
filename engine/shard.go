package engine

import (
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/rs/zerolog"
)

// ShardManager 分片管理器
type ShardManager struct {
	shards    []*MatchingEngine
	numShards int
	symbolMap map[string]int // 交易对 -> 分片索引
	mu        sync.RWMutex
	logger    zerolog.Logger
}

// NewShardManager 创建分片管理器
func NewShardManager(numShards int, symbols []string, logger zerolog.Logger) *ShardManager {
	sm := &ShardManager{
		shards:    make([]*MatchingEngine, numShards),
		numShards: numShards,
		symbolMap: make(map[string]int),
		logger:    logger,
	}

	// 初始化每个分片的引擎
	for i := 0; i < numShards; i++ {
		sm.shards[i] = NewMatchingEngine(fmt.Sprintf("shard-%d", i), logger.With().Int("shard", i).Logger())
	}

	// 分配交易对到分片
	for _, symbol := range symbols {
		shard := sm.getShardIndex(symbol)
		sm.symbolMap[symbol] = shard
		sm.logger.Info().
			Str("symbol", symbol).
			Int("shard", shard).
			Msg("Symbol assigned to shard")
	}

	return sm
}

// getShardIndex 获取分片索引
func (sm *ShardManager) getShardIndex(symbol string) int {
	h := fnv.New32a()
	h.Write([]byte(symbol))
	return int(h.Sum32()) % sm.numShards
}

// GetEngine 获取交易对对应的撮合引擎
func (sm *ShardManager) GetEngine(symbol string) *MatchingEngine {
	sm.mu.RLock()
	shard, ok := sm.symbolMap[symbol]
	sm.mu.RUnlock()

	if !ok {
		return nil
	}

	return sm.shards[shard]
}

// SubmitOrder 提交订单到对应分片
func (sm *ShardManager) SubmitOrder(order *Order) error {
	engine := sm.GetEngine(order.Symbol)
	if engine == nil {
		return &EngineError{Code: "SYMBOL_NOT_FOUND", Message: "Symbol not registered"}
	}
	return engine.SubmitOrder(order)
}

// GetAllStats 获取所有分片统计
func (sm *ShardManager) GetAllStats() []EngineStats {
	stats := make([]EngineStats, sm.numShards)

	for i, engine := range sm.shards {
		stats[i] = engine.GetStats()
	}

	return stats
}

// GetSymbolsByShard 获取指定分片的所有交易对
func (sm *ShardManager) GetSymbolsByShard(shard int) []string {
	var symbols []string

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for symbol, s := range sm.symbolMap {
		if s == shard {
			symbols = append(symbols, symbol)
		}
	}

	return symbols
}

// Stop 停止所有分片
func (sm *ShardManager) Stop() {
	for _, engine := range sm.shards {
		engine.Stop()
	}
}

// ShardAwareRouter 分片感知路由
type ShardAwareRouter struct {
	manager *ShardManager
	mu      sync.RWMutex
}

// NewShardAwareRouter 创建分片感知路由
func NewShardAwareRouter(numShards int, symbols []string, logger zerolog.Logger) *ShardAwareRouter {
	return &ShardAwareRouter{
		manager: NewShardManager(numShards, symbols, logger),
	}
}

// RouteOrder 路由订单
func (r *ShardAwareRouter) RouteOrder(order *Order) error {
	return r.manager.SubmitOrder(order)
}

// GetEngine 获取引擎
func (r *ShardAwareRouter) GetEngine(symbol string) *MatchingEngine {
	return r.manager.GetEngine(symbol)
}

// Stop 停止
func (r *ShardAwareRouter) Stop() {
	r.manager.Stop()
}

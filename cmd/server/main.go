package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/smartx/matching-engine/api"
	"github.com/smartx/matching-engine/config"
	"github.com/smartx/matching-engine/engine"
	"github.com/smartx/matching-engine/engine/futures"
	"github.com/smartx/matching-engine/risk"
	"github.com/smartx/matching-engine/ws"
)

var SYMBOLS = []string{
	"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "DOGEUSDT",
	"XRPUSDT", "DOTUSDT", "UNIUSDT", "LTCUSDT", "LINKUSDT",
	"MATICUSDT", "SOLUSDT", "AVAXUSDT", "ATOMUSDT", "FILUSDT",
}

var FUTURES_SYMBOLS = []string{
	"BTCUSDT_PERP", "ETHUSDT_PERP", "BNBUSDT_PERP",
	"SOLUSDT_PERP", "AVAXUSDT_PERP", "XRPUSDT_PERP",
}

// main 服务入口函数
// 初始化合约撮合引擎、风险管理模块、API处理器和WebSocket服务
// [Design: 系统架构](../DESIGN_DERIVATIVES.md#21-整体架构图)
func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	shards := flag.Int("shards", 8, "Number of matching engine shards")
	flag.Parse()

	initLogger()

	log.Info().Msg("SmartX Matching Engine starting...")

	cfg := config.Load()

	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *shards > 0 {
		cfg.Matching.Shards = *shards
	}

	shardRouter := engine.NewShardAwareRouter(cfg.Matching.Shards, SYMBOLS, log.Logger)

	wsConfig := &ws.WSConfig{
		ReadBufferSize:  cfg.Server.WS.ReadBufferSize,
		WriteBufferSize: cfg.Server.WS.WriteBufferSize,
		MaxMessageSize:  cfg.Server.WS.MaxMessageSize,
		PingInterval:    time.Duration(cfg.Server.WS.PingInterval) * time.Second,
		PongTimeout:     time.Duration(cfg.Server.WS.PongTimeout) * time.Second,
		SendBufferSize:  cfg.Matching.BufferSize,
	}
	wsHub := ws.NewHub(wsConfig, log.Logger)

	go wsHub.Run()

	for _, symbol := range SYMBOLS {
		broadcaster := ws.NewMarketDataBroadcaster(wsHub, symbol, time.Second, log.Logger)
		broadcaster.Start()
	}

	go startRealtimeBroadcast(shardRouter, wsHub)

	// 初始化合约撮合引擎
	futuresEngines := make(map[string]*futures.FuturesEngine)
	for _, symbol := range FUTURES_SYMBOLS {
		fe := futures.NewFuturesEngine(symbol, log.Logger)
		futuresEngines[symbol] = fe
	}

	// 初始化保证金引擎
	marginEngine := risk.NewMarginEngine()

	// 初始化标记价格引擎
	markPriceEngines := make(map[string]*risk.MarkPriceEngine)
	for _, symbol := range FUTURES_SYMBOLS {
		mpe := risk.NewMarkPriceEngine(symbol, log.Logger)
		markPriceEngines[symbol] = mpe
	}

	// 初始化资金费率引擎
	fundingRateEngines := make(map[string]*risk.FundingRateEngine)
	for _, symbol := range FUTURES_SYMBOLS {
		fre := risk.NewFundingRateEngine(symbol, markPriceEngines[symbol], marginEngine, log.Logger)
		fre.CalculateFundingRate()
		fundingRateEngines[symbol] = fre
	}

	// 初始化强平引擎
	liquidationEngines := make(map[string]*risk.LiquidationEngine)
	for _, symbol := range FUTURES_SYMBOLS {
		le := risk.NewLiquidationEngine(marginEngine, log.Logger)
		liquidationEngines[symbol] = le
	}

	// 初始化合约API处理器
	futuresHandlers := make(map[string]*api.FuturesHandler)
	for _, symbol := range FUTURES_SYMBOLS {
		fh := api.NewFuturesHandler(
			futuresEngines[symbol],
			marginEngine,
			liquidationEngines[symbol],
			markPriceEngines[symbol],
			fundingRateEngines[symbol],
			log.Logger,
		)
		futuresHandlers[symbol] = fh
	}

	handler := api.NewHandler(shardRouter, wsHub, log.Logger, futuresHandlers)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("HTTP server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	go startMetricsServer(9090)

	go startFuturesMarketSimulator(futuresEngines, markPriceEngines, marginEngine, liquidationEngines)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	shardRouter.Stop()

	for _, fe := range futuresEngines {
		fe.Stop()
	}

	wsHub.Stop()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited")
}

// initLogger 初始化日志配置
func initLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	log.Logger = zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Logger()
}

// startRealtimeBroadcast 启动实时行情广播
func startRealtimeBroadcast(router *engine.ShardAwareRouter, hub *ws.Hub) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		for _, symbol := range SYMBOLS {
			eng := router.GetEngine(symbol)
			if eng == nil {
				continue
			}

			bids, asks := eng.GetDepth(20)

			if len(bids) > 0 || len(asks) > 0 {
				hub.PublishOrderBook(symbol, map[string]interface{}{
					"symbol": symbol,
					"bids":   bids,
					"asks":   asks,
					"time":   time.Now().UnixMilli(),
				})
			}

			trades := eng.GetTrades()
			for _, trade := range trades {
				hub.PublishTrade(symbol, map[string]interface{}{
					"e": "trade",
					"E": time.Now().UnixMilli(),
					"s": symbol,
					"t": trade.TradeID,
					"p": trade.Price,
					"q": trade.Quantity,
					"T": trade.Timestamp,
					"m": trade.Side == engine.Sell,
				})
			}

			bestBid, bestAsk, bidQty, askQty := eng.GetBestBidAsk()
			hub.PublishTicker(symbol, map[string]interface{}{
				"e": "24hrTicker",
				"E": time.Now().UnixMilli(),
				"s": symbol,
				"c": bestAsk,
				"b": bestBid,
				"a": bestAsk,
				"B": bidQty,
				"A": askQty,
				"v": 0,
				"q": 0,
				"h": bestAsk,
				"l": bestBid,
				"P": 0,
				"p": 0,
				"w": 0,
				"x": bestBid,
				"C": 0,
				"Q": 0,
				"F": 0,
				"L": 0,
				"n": 0,
			})
		}
	}
}

// startFuturesMarketSimulator 启动合约市场模拟器
// 模拟价格波动，更新标记价格和强平价格
func startFuturesMarketSimulator(futuresEngines map[string]*futures.FuturesEngine,
	markPriceEngines map[string]*risk.MarkPriceEngine, marginEngine *risk.MarginEngine,
	liquidationEngines map[string]*risk.LiquidationEngine) {

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	basePrices := map[string]float64{
		"BTCUSDT_PERP": 94250.50,
		"ETHUSDT_PERP": 4850.25,
		"BNBUSDT_PERP": 610.75,
		"SOLUSDT_PERP": 175.50,
		"AVAXUSDT_PERP": 35.25,
		"XRPUSDT_PERP": 0.6250,
	}

	priceOffsets := map[string]float64{}
	for _, symbol := range FUTURES_SYMBOLS {
		priceOffsets[symbol] = 0
	}

	for range ticker.C {
		for _, symbol := range FUTURES_SYMBOLS {
			basePrice := basePrices[symbol]
			priceOffsets[symbol] += (float64(time.Now().UnixNano()%1000)-500) * 0.0001
			currentPrice := basePrice * (1 + priceOffsets[symbol])

			if mpe, ok := markPriceEngines[symbol]; ok {
				mpe.SetExchangePrice("binance", currentPrice)
				mpe.SetExchangePrice("okx", currentPrice*1.0001)
				mpe.SetExchangePrice("coinbase", currentPrice*0.9999)
				mpe.CalculateMarkPrice(0, 8*time.Hour)
			}

			if fe, ok := futuresEngines[symbol]; ok {
				ob := fe.GetOrderBook()
				if ob.LastPrice > 0 {
					if mpe, ok := markPriceEngines[symbol]; ok {
						mpe.UpdateLastTradePrice(ob.LastPrice)
					}
				}
			}

			if le, ok := liquidationEngines[symbol]; ok {
				le.UpdateAllLiquidationPrices(symbol, currentPrice)
			}
		}
	}
}

// startMetricsServer 启动指标监控服务器
func startMetricsServer(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`
# HELP matching_engine_uptime Engine uptime
# TYPE matching_engine_uptime gauge
matching_engine_uptime %d

# HELP matching_engine_symbols_total Total symbols
# TYPE matching_engine_symbols_total gauge
matching_engine_symbols_total %d

# HELP matching_engine_futures_symbols_total Total futures symbols
# TYPE matching_engine_futures_symbols_total gauge
matching_engine_futures_symbols_total %d
`, time.Now().Unix(), len(SYMBOLS), len(FUTURES_SYMBOLS))))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Info().Str("addr", addr).Msg("Metrics server starting")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error().Err(err).Msg("Metrics server failed")
	}
}
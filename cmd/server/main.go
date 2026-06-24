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
	"github.com/smartx/matching-engine/ws"
)

// 支持的交易对列表
var SYMBOLS = []string{
	"BTCUSDT", "ETHUSDT", "BNBUSDT", "ADAUSDT", "DOGEUSDT",
	"XRPUSDT", "DOTUSDT", "UNIUSDT", "LTCUSDT", "LINKUSDT",
	"MATICUSDT", "SOLUSDT", "AVAXUSDT", "ATOMUSDT", "FILUSDT",
}

func main() {
	// 解析命令行参数
	port := flag.Int("port", 8080, "HTTP server port")
	shards := flag.Int("shards", 8, "Number of matching engine shards")
	flag.Parse()

	// 初始化日志
	initLogger()

	log.Info().Msg("SmartX Matching Engine starting...")

	// 加载配置
	cfg := config.Load()

	// 覆盖配置
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *shards > 0 {
		cfg.Matching.Shards = *shards
	}

	// 创建分片路由
	shardRouter := engine.NewShardAwareRouter(cfg.Matching.Shards, SYMBOLS, log.Logger)

	// 创建WebSocket中心
	wsConfig := &ws.WSConfig{
		ReadBufferSize:  cfg.Server.WS.ReadBufferSize,
		WriteBufferSize: cfg.Server.WS.WriteBufferSize,
		MaxMessageSize:  cfg.Server.WS.MaxMessageSize,
		PingInterval:    time.Duration(cfg.Server.WS.PingInterval) * time.Second,
		PongTimeout:     time.Duration(cfg.Server.WS.PongTimeout) * time.Second,
		SendBufferSize:  cfg.Matching.BufferSize,
	}
	wsHub := ws.NewHub(wsConfig, log.Logger)

	// 启动WebSocket Hub
	go wsHub.Run()

	// 创建行情广播器
	for _, symbol := range SYMBOLS {
		broadcaster := ws.NewMarketDataBroadcaster(wsHub, symbol, time.Second, log.Logger)
		broadcaster.Start()
	}

	// 创建API处理器
	handler := api.NewHandler(shardRouter, wsHub, log.Logger)

	// 启动HTTP服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler.Handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 启动服务器
	go func() {
		log.Info().Str("addr", addr).Msg("HTTP server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// 启动性能监控
	go startMetricsServer(9090)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 停止分片路由
	shardRouter.Stop()

	// 停止WebSocket Hub
	wsHub.Stop()

	// 关闭HTTP服务器
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited")
}

// initLogger 初始化日志
func initLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// 设置日志级别
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

// startMetricsServer 启动性能监控服务器
func startMetricsServer(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		// 简单的性能指标输出
		w.Write([]byte(fmt.Sprintf(`
# HELP matching_engine_uptime Engine uptime
# TYPE matching_engine_uptime gauge
matching_engine_uptime %d

# HELP matching_engine_symbols_total Total symbols
# TYPE matching_engine_symbols_total gauge
matching_engine_symbols_total %d
`, time.Now().Unix(), len(SYMBOLS))))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Info().Str("addr", addr).Msg("Metrics server starting")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Error().Err(err).Msg("Metrics server failed")
	}
}

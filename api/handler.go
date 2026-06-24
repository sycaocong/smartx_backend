package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/smartx/matching-engine/engine"
	"github.com/smartx/matching-engine/ws"
)

// Handler API处理器
type Handler struct {
	engine *engine.ShardAwareRouter
	wsHub  *ws.Hub
	logger zerolog.Logger
	server *http.Server
}

// NewHandler 创建API处理器
func NewHandler(engine *engine.ShardAwareRouter, wsHub *ws.Hub, logger zerolog.Logger) *Handler {
	h := &Handler{
		engine: engine,
		wsHub:  wsHub,
		logger: logger,
	}

	mux := http.NewServeMux()
	h.setupRoutes(mux)

	h.server = &http.Server{
		Handler: mux,
	}

	return h
}

// Handler 返回HTTP处理器
func (h *Handler) Handler() http.Handler {
	return h.server.Handler
}

// setupRoutes 设置路由
func (h *Handler) setupRoutes(mux *http.ServeMux) {
	// 健康检查
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ready", h.Ready)

	// WebSocket
	mux.HandleFunc("/ws", h.WebSocket)

	// 订单接口
	mux.HandleFunc("/api/v1/orders", h.handleOrders)
	mux.HandleFunc("/api/v1/orders/", h.handleOrder)

	// 市场数据接口
	mux.HandleFunc("/api/v1/market/ticker/", h.GetTicker)
	mux.HandleFunc("/api/v1/market/orderbook/", h.GetOrderBook)
	mux.HandleFunc("/api/v1/market/trades/", h.GetTrades)
	mux.HandleFunc("/api/v1/market/kline/", h.GetKLine)
	mux.HandleFunc("/api/v1/market/depth/", h.GetDepth)

	// 统计接口
	mux.HandleFunc("/api/v1/stats", h.GetStats)
	mux.HandleFunc("/api/v1/stats/shard", h.GetShardStats)
}

// Start 启动HTTP服务器
func (h *Handler) Start(addr string) error {
	h.server.Addr = addr
	h.logger.Info().Str("addr", addr).Msg("HTTP server starting")
	return h.server.ListenAndServe()
}

// Stop 停止HTTP服务器
func (h *Handler) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// CORS 中间件
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// 日志中间件
func logMiddleware(logger zerolog.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next(wrapped, r)

		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", wrapped.statusCode).
			Dur("latency", time.Since(start)).
			Msg("HTTP request")
	}
}

// responseWriter 响应写入器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Health 健康检查
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Ready 就绪检查
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

// WebSocket WebSocket连接
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleWebSocket(w, r)
}

// handleOrders 处理订单相关请求
func (h *Handler) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		h.CreateOrder(w, r)
	case "GET":
		h.GetOrders(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleOrder 处理单个订单请求
func (h *Handler) handleOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/orders/")
	if len(path) == 0 {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		h.GetOrder(w, r, path)
	case "DELETE":
		h.CancelOrder(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// CreateOrder 创建订单
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Symbol        string  `json:"symbol"`
		Side          string  `json:"side"`
		Type          string  `json:"type"`
		Price         float64 `json:"price"`
		Quantity      float64 `json:"quantity"`
		ClientOrderID string  `json:"client_order_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证请求
	if req.Symbol == "" || req.Side == "" || req.Type == "" || req.Quantity <= 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// 转换为引擎格式
	side := engine.Buy
	if req.Side == "SELL" {
		side = engine.Sell
	}

	orderType := engine.LimitOrder
	if req.Type == "MARKET" {
		orderType = engine.MarketOrder
	}

	// 创建订单
	order := engine.NewOrder(req.Symbol, side, orderType, req.Price, req.Quantity, req.ClientOrderID)

	// 提交到撮合引擎
	if err := h.engine.RouteOrder(order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回订单信息
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order.ToProto())
}

// GetOrder 获取订单
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "not_implemented",
	})
}

// CancelOrder 取消订单
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "canceled",
	})
}

// GetOrders 获取订单列表
func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limit := r.URL.Query().Get("limit")

	if limit == "" {
		limit = "100"
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": []interface{}{},
		"limit":  limit,
		"symbol": symbol,
	})
}

// GetTicker 获取Ticker
func (h *Handler) GetTicker(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/ticker/")

	eng := h.engine.GetEngine(symbol)
	if eng == nil {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	stats := eng.GetStats()
	bids, asks, bidQty, askQty := eng.GetOrderBook().GetBestBidAsk()

	ticker := &ws.TickerData{
		Symbol:    symbol,
		LastPrice: stats.LastPrice,
		BidPrice:  bids,
		AskPrice:  asks,
		BidQty:    bidQty,
		AskQty:    askQty,
		Timestamp: time.Now().UnixMilli(),
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ticker)
}

// GetOrderBook 获取订单簿
func (h *Handler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/orderbook/")

	eng := h.engine.GetEngine(symbol)
	if eng == nil {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	bids, asks := eng.GetDepth(limit)

	ob := &ws.OrderBookData{
		Symbol:    symbol,
		Version:   time.Now().UnixNano(),
		Timestamp: time.Now().UnixMilli(),
	}

	for _, bid := range bids {
		ob.Bids = append(ob.Bids, []interface{}{bid.Price, bid.Quantity})
	}
	for _, ask := range asks {
		ob.Asks = append(ob.Asks, []interface{}{ask.Price, ask.Quantity})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ob)
}

// GetTrades 获取成交历史
func (h *Handler) GetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/trades/")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trades": []interface{}{},
		"symbol": symbol,
	})
}

// GetKLine 获取K线
func (h *Handler) GetKLine(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/kline/")

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1m"
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":   symbol,
		"interval": interval,
		"klines":   []interface{}{},
		"limit":    limit,
	})
}

// GetDepth 获取深度
func (h *Handler) GetDepth(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/depth/")

	eng := h.engine.GetEngine(symbol)
	if eng == nil {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	bids, asks := eng.GetDepth(limit)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol": symbol,
		"bids":   bids,
		"asks":   asks,
	})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	wsStats := h.wsHub.GetStats()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ws_clients":  wsStats.ClientCount,
		"ws_messages": wsStats.MessageCount,
		"timestamp":   time.Now().UnixMilli(),
	})
}

// GetShardStats 获取分片统计
func (h *Handler) GetShardStats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"shards": []interface{}{},
	})
}

// Error 错误处理
func (h *Handler) Error(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

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

type Handler struct {
	engine          *engine.ShardAwareRouter
	wsHub           *ws.Hub
	logger          zerolog.Logger
	server          *http.Server
	futuresHandlers map[string]*FuturesHandler
}

func NewHandler(engine *engine.ShardAwareRouter, wsHub *ws.Hub, logger zerolog.Logger,
	futuresHandlers ...map[string]*FuturesHandler) *Handler {

	h := &Handler{
		engine: engine,
		wsHub:  wsHub,
		logger: logger,
	}

	if len(futuresHandlers) > 0 {
		h.futuresHandlers = futuresHandlers[0]
	}

	mux := http.NewServeMux()
	h.setupRoutes(mux)

	h.server = &http.Server{
		Handler: mux,
	}

	return h
}

func (h *Handler) Handler() http.Handler {
	return h.server.Handler
}

func (h *Handler) setupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ready", h.Ready)

	mux.HandleFunc("/ws", h.WebSocket)

	mux.HandleFunc("/api/v1/orders", h.handleOrders)
	mux.HandleFunc("/api/v1/orders/", h.handleOrder)

	mux.HandleFunc("/api/v1/market/ticker/", h.GetTicker)
	mux.HandleFunc("/api/v1/market/orderbook/", h.GetOrderBook)
	mux.HandleFunc("/api/v1/market/trades/", h.GetTrades)
	mux.HandleFunc("/api/v1/market/kline/", h.GetKLine)
	mux.HandleFunc("/api/v1/market/depth/", h.GetDepth)

	mux.HandleFunc("/api/v1/stats", h.GetStats)
	mux.HandleFunc("/api/v1/stats/shard", h.GetShardStats)

	if h.futuresHandlers != nil {
		h.setupFuturesRoutes(mux)
	}
}

func (h *Handler) setupFuturesRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/futures/orders", h.handleFuturesOrders)
	mux.HandleFunc("/api/v1/futures/orders/", h.handleFuturesOrder)
	mux.HandleFunc("/api/v1/futures/positions", h.handleFuturesPositions)
	mux.HandleFunc("/api/v1/futures/funding-rate", h.handleFuturesFundingRate)
	mux.HandleFunc("/api/v1/futures/mark-price", h.handleFuturesMarkPrice)
	mux.HandleFunc("/api/v1/futures/ticker", h.handleFuturesTicker)
	mux.HandleFunc("/api/v1/futures/orderbook", h.handleFuturesOrderBook)
}

func (h *Handler) handleFuturesOrders(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.handleFuturesOrders(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/futures/orders/")
	if len(path) == 0 {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.handleFuturesOrder(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesPositions(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.handleFuturesPositions(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesFundingRate(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.GetFundingRate(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesMarkPrice(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.GetMarkPrice(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.GetTicker(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) handleFuturesOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT_PERP"
	}

	if fh, ok := h.futuresHandlers[symbol]; ok {
		fh.GetOrderBook(w, r)
		return
	}

	http.Error(w, "Futures symbol not found", http.StatusNotFound)
}

func (h *Handler) Start(addr string) error {
	h.server.Addr = addr
	h.logger.Info().Str("addr", addr).Msg("HTTP server starting")
	return h.server.ListenAndServe()
}

func (h *Handler) Stop(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

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

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsHub.HandleWebSocket(w, r)
}

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

	if req.Symbol == "" || req.Side == "" || req.Type == "" || req.Quantity <= 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	side := engine.Buy
	if req.Side == "SELL" {
		side = engine.Sell
	}

	orderType := engine.LimitOrder
	if req.Type == "MARKET" {
		orderType = engine.MarketOrder
	}

	order := engine.NewOrder(req.Symbol, side, orderType, req.Price, req.Quantity, req.ClientOrderID)

	if err := h.engine.RouteOrder(order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order.ToProto())
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "not_implemented",
	})
}

func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "canceled",
	})
}

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

func (h *Handler) GetTicker(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/ticker/")

	eng := h.engine.GetEngine(symbol)
	if eng == nil {
		http.Error(w, "Symbol not found", http.StatusNotFound)
		return
	}

	stats := eng.GetStats()
	bids, asks, bidQty, askQty := eng.GetOrderBook().GetBestBidAsk()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":    symbol,
		"last_price": stats.LastPrice,
		"bid_price":  bids,
		"ask_price":  asks,
		"bid_qty":    bidQty,
		"ask_qty":    askQty,
		"timestamp":  time.Now().UnixMilli(),
	})
}

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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":    symbol,
		"version":   time.Now().UnixNano(),
		"timestamp": time.Now().UnixMilli(),
		"bids":      bids,
		"asks":      asks,
	})
}

func (h *Handler) GetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/v1/market/trades/")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"trades": []interface{}{},
		"symbol": symbol,
	})
}

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

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	wsStats := h.wsHub.GetStats()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ws_clients":  wsStats.ClientCount,
		"ws_messages": wsStats.MessageCount,
		"timestamp":   time.Now().UnixMilli(),
	})
}

func (h *Handler) GetShardStats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"shards": []interface{}{},
	})
}

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
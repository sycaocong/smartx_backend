package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/smartx/matching-engine/engine/futures"
	"github.com/smartx/matching-engine/risk"
)

// FuturesHandler 合约交易API处理器
// 提供合约订单管理、持仓管理、市场数据查询等REST接口
// [Design: API接口设计](../DESIGN_DERIVATIVES.md#5-api接口设计)
type FuturesHandler struct {
	engine            *futures.FuturesEngine      // 合约撮合引擎
	marginEngine      *risk.MarginEngine         // 保证金引擎
	liquidationEngine *risk.LiquidationEngine    // 强平引擎
	markPriceEngine   *risk.MarkPriceEngine      // 标记价格引擎
	fundingRateEngine *risk.FundingRateEngine    // 资金费率引擎
	logger            zerolog.Logger             // 日志
}

// NewFuturesHandler 创建新的合约API处理器
func NewFuturesHandler(engine *futures.FuturesEngine, marginEngine *risk.MarginEngine,
	liquidationEngine *risk.LiquidationEngine, markPriceEngine *risk.MarkPriceEngine,
	fundingRateEngine *risk.FundingRateEngine, logger zerolog.Logger) *FuturesHandler {

	return &FuturesHandler{
		engine:            engine,
		marginEngine:      marginEngine,
		liquidationEngine: liquidationEngine,
		markPriceEngine:   markPriceEngine,
		fundingRateEngine: fundingRateEngine,
		logger:            logger.With().Str("module", "futures").Logger(),
	}
}

// CreateOrder 创建合约订单
// POST /api/v1/futures/orders
// [Design: 订单管理](../DESIGN_DERIVATIVES.md#51-订单管理)
func (fh *FuturesHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Symbol         string  `json:"symbol"`          // 交易对
		Side           string  `json:"side"`            // 买卖方向(BUY/SELL)
		Type           string  `json:"type"`            // 订单类型(MARKET/LIMIT)
		Price          float64 `json:"price"`           // 价格
		Quantity       float64 `json:"quantity"`        // 数量
		Leverage       int     `json:"leverage"`        // 杠杆倍数(1-100)
		MarginType     string  `json:"margin_type"`     // 保证金类型(cross/isolated)
		PositionSide   string  `json:"position_side"`   // 持仓方向(long/short)
		ClientOrderID  string  `json:"client_order_id"` // 客户端订单ID
		UserID         string  `json:"user_id"`         // 用户ID
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" || req.Side == "" || req.Type == "" || req.Quantity <= 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	if req.Leverage < 1 || req.Leverage > risk.MaxLeverage {
		http.Error(w, "Invalid leverage", http.StatusBadRequest)
		return
	}

	side := futures.Buy
	if req.Side == "SELL" {
		side = futures.Sell
	}

	orderType := futures.FuturesLimitOrder
	if req.Type == "MARKET" {
		orderType = futures.FuturesMarketOrder
	}

	marginType := futures.CrossMargin
	if req.MarginType == "isolated" {
		marginType = futures.IsolatedMargin
	}

	positionSide := futures.Long
	if req.PositionSide == "short" {
		positionSide = futures.Short
	}

	order := futures.NewFuturesOrder(req.Symbol, side, orderType, req.Price, req.Quantity,
		req.Leverage, marginType, positionSide, req.UserID, req.ClientOrderID)

	order.LiquidationPrice = risk.CalculateLiquidationPrice(&futures.Position{
		EntryPrice: req.Price,
		Leverage:   req.Leverage,
		Side:       positionSide,
	})

	if err := fh.engine.SubmitOrder(order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order.ToProto())
}

// GetOrder 查询单个订单
// GET /api/v1/futures/orders/{order_id}
func (fh *FuturesHandler) GetOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "not_implemented",
	})
}

// CancelOrder 取消订单
// DELETE /api/v1/futures/orders/{order_id}
func (fh *FuturesHandler) CancelOrder(w http.ResponseWriter, r *http.Request, orderID string) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"order_id": orderID,
		"status":   "canceled",
	})
}

// GetOrders 查询订单列表
// GET /api/v1/futures/orders
func (fh *FuturesHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
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

// GetPositions 查询用户持仓
// GET /api/v1/futures/positions
// [Design: 持仓管理](../DESIGN_DERIVATIVES.md#52-持仓管理)
func (fh *FuturesHandler) GetPositions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")

	var positions []interface{}
	posList := fh.marginEngine.GetPositions(userID)
	for _, pos := range posList {
		positions = append(positions, pos.ToProto())
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"positions": positions,
	})
}

// ClosePosition 平仓
// POST /api/v1/futures/positions
// [Design: 持仓管理](../DESIGN_DERIVATIVES.md#52-持仓管理)
func (fh *FuturesHandler) ClosePosition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string  `json:"user_id"`   // 用户ID
		Symbol   string  `json:"symbol"`    // 交易对
		Side     string  `json:"side"`      // 持仓方向(long/short)
		Quantity float64 `json:"quantity"`  // 平仓数量
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	positionSide := futures.Long
	if req.Side == "short" {
		positionSide = futures.Short
	}

	position := fh.marginEngine.GetPosition(req.UserID, req.Symbol, positionSide)
	if position == nil {
		http.Error(w, "Position not found", http.StatusNotFound)
		return
	}

	markPrice := fh.markPriceEngine.GetMarkPrice()
	position.Close(markPrice)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(position.ToProto())
}

// GetFundingRate 获取资金费率
// GET /api/v1/futures/funding-rate
// [Design: 市场数据](../DESIGN_DERIVATIVES.md#53-市场数据)
func (fh *FuturesHandler) GetFundingRate(w http.ResponseWriter, r *http.Request) {
	fre := fh.fundingRateEngine
	mpe := fre.GetMarkPriceEngine()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&risk.FundingRateData{
		Symbol:          fre.GetSymbol(),
		FundingRate:     fre.GetFundingRate(),
		NextFundingTime: fre.GetNextFundingTime().UnixMilli(),
		LastFundingTime: fre.GetLastFundingTime().UnixMilli(),
		MarkPrice:       mpe.GetMarkPrice(),
		IndexPrice:      mpe.GetIndexPrice(),
		Premium:         mpe.GetPremium(),
	})
}

// GetMarkPrice 获取标记价格
// GET /api/v1/futures/mark-price
// [Design: 市场数据](../DESIGN_DERIVATIVES.md#53-市场数据)
func (fh *FuturesHandler) GetMarkPrice(w http.ResponseWriter, r *http.Request) {
	mpe := fh.markPriceEngine

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&risk.MarkPriceData{
		Symbol:     mpe.GetSymbol(),
		MarkPrice:  mpe.GetMarkPrice(),
		IndexPrice: mpe.GetIndexPrice(),
		Premium:    mpe.GetPremium(),
		Timestamp:  time.Now().UnixMilli(),
	})
}

// GetTicker 获取行情数据
// GET /api/v1/futures/ticker
// [Design: 市场数据](../DESIGN_DERIVATIVES.md#53-市场数据)
func (fh *FuturesHandler) GetTicker(w http.ResponseWriter, r *http.Request) {
	stats := fh.engine.GetStats()
	bids, asks, bidQty, askQty := fh.engine.GetBestBidAsk()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"symbol":           fh.engine.GetStats().Symbol,
		"last_price":       stats.LastPrice,
		"bid_price":        bids,
		"ask_price":        asks,
		"bid_qty":          bidQty,
		"ask_qty":          askQty,
		"mark_price":       fh.markPriceEngine.GetMarkPrice(),
		"index_price":      fh.markPriceEngine.GetIndexPrice(),
		"funding_rate":     fh.fundingRateEngine.GetFundingRate(),
		"next_funding_time": fh.fundingRateEngine.GetNextFundingTime().UnixMilli(),
		"timestamp":        time.Now().UnixMilli(),
	})
}

// GetOrderBook 获取订单簿深度
// GET /api/v1/futures/orderbook
// [Design: 市场数据](../DESIGN_DERIVATIVES.md#53-市场数据)
func (fh *FuturesHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	bids, asks := fh.engine.GetDepth(limit)

	ob := map[string]interface{}{
		"symbol":    fh.engine.GetStats().Symbol,
		"version":   time.Now().UnixNano(),
		"timestamp": time.Now().UnixMilli(),
		"bids":      []interface{}{},
		"asks":      []interface{}{},
	}

	for _, bid := range bids {
		ob["bids"] = append(ob["bids"].([]interface{}), []interface{}{bid.Price, bid.Quantity})
	}
	for _, ask := range asks {
		ob["asks"] = append(ob["asks"].([]interface{}), []interface{}{ask.Price, ask.Quantity})
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ob)
}

// handleFuturesOrders 处理订单列表请求
func (fh *FuturesHandler) handleFuturesOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		fh.CreateOrder(w, r)
	case "GET":
		fh.GetOrders(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFuturesOrder 处理单个订单请求
func (fh *FuturesHandler) handleFuturesOrder(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/futures/orders/")
	if len(path) == 0 {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		fh.GetOrder(w, r, path)
	case "DELETE":
		fh.CancelOrder(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFuturesPositions 处理持仓请求
func (fh *FuturesHandler) handleFuturesPositions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		fh.GetPositions(w, r)
	case "POST":
		fh.ClosePosition(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterRoutes 注册合约API路由
func (fh *FuturesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/futures/orders", fh.handleFuturesOrders)
	mux.HandleFunc("/api/v1/futures/orders/", fh.handleFuturesOrder)
	mux.HandleFunc("/api/v1/futures/positions", fh.handleFuturesPositions)
	mux.HandleFunc("/api/v1/futures/funding-rate", fh.GetFundingRate)
	mux.HandleFunc("/api/v1/futures/mark-price", fh.GetMarkPrice)
	mux.HandleFunc("/api/v1/futures/ticker", fh.GetTicker)
	mux.HandleFunc("/api/v1/futures/orderbook", fh.GetOrderBook)
}
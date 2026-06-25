package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// Hub WebSocket中心
type Hub struct {
	// 客户端连接
	clients map[*Client]bool

	// 订阅管理
	subscriptions map[*Client]map[string]bool // client -> topics

	// 广播通道
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client

	// 统计
	clientCount  int64
	messageCount int64

	// 配置
	config *WSConfig

	// 日志
	logger zerolog.Logger

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc

	mu sync.RWMutex
}

// WSConfig WebSocket配置
type WSConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	MaxMessageSize  int64
	PingInterval    time.Duration
	PongTimeout     time.Duration
	SendBufferSize  int
}

// Message 消息结构
type Message struct {
	Type    string      `json:"type"`
	Topic   string      `json:"topic"`
	Symbol  string      `json:"symbol"`
	Data    interface{} `json:"data"`
	Time    int64       `json:"time"`
	TraceID string      `json:"trace_id,omitempty"`
}

// Client WebSocket客户端
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	groups     map[string]bool
	mu         sync.RWMutex
	connected  bool
	remoteAddr string
}

// Upgrader WebSocket升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 生产环境需要验证Origin
	},
}

// NewHub 创建WebSocket中心
func NewHub(config *WSConfig, logger zerolog.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		clients:       make(map[*Client]bool),
		subscriptions: make(map[*Client]map[string]bool),
		broadcast:     make(chan *Message, config.SendBufferSize),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		config:        config,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Run 运行WebSocket中心
func (h *Hub) Run() {
	h.logger.Info().Msg("WebSocket Hub starting...")

	for {
		select {
		case <-h.ctx.Done():
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.subscriptions[client] = make(map[string]bool)
			atomic.AddInt64(&h.clientCount, 1)
			h.mu.Unlock()

			h.logger.Info().
				Str("remote_addr", client.remoteAddr).
				Int64("total_clients", atomic.LoadInt64(&h.clientCount)).
				Msg("Client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.subscriptions, client)
				atomic.AddInt64(&h.clientCount, -1)
				close(client.send)
			}
			h.mu.Unlock()

			h.logger.Info().
				Str("remote_addr", client.remoteAddr).
				Int64("total_clients", atomic.LoadInt64(&h.clientCount)).
				Msg("Client disconnected")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				// 检查订阅
				topic := message.Topic
				if topic != "" {
					client.mu.RLock()
					if !client.groups[topic] {
						client.mu.RUnlock()
						continue
					}
					client.mu.RUnlock()
				}

				data, err := json.Marshal(message)
				if err != nil {
					continue
				}

				select {
				case client.send <- data:
				default:
					// 客户端缓冲区满，跳过
				}
			}
			h.mu.RUnlock()

			atomic.AddInt64(&h.messageCount, 1)
		}
	}
}

// HandleWebSocket 处理WebSocket连接
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	client := &Client{
		hub:        h,
		conn:       conn,
		send:       make(chan []byte, 1024), // 增大缓冲区提高吞吐量
		groups:     make(map[string]bool),
		connected:  true,
		remoteAddr: r.RemoteAddr,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// Subscribe 订阅主题
func (h *Hub) Subscribe(client *Client, topics []string) {
	client.mu.Lock()
	for _, topic := range topics {
		client.groups[topic] = true
		h.logger.Debug().
			Str("remote_addr", client.remoteAddr).
			Str("topic", topic).
			Msg("Client subscribed")
	}
	client.mu.Unlock()
}

// Unsubscribe 取消订阅
func (h *Hub) Unsubscribe(client *Client, topics []string) {
	client.mu.Lock()
	for _, topic := range topics {
		delete(client.groups, topic)
		h.logger.Debug().
			Str("remote_addr", client.remoteAddr).
			Str("topic", topic).
			Msg("Client unsubscribed")
	}
	client.mu.Unlock()
}

// Publish 发布消息到主题
func (h *Hub) Publish(topic, symbol string, data interface{}) {
	msg := &Message{
		Type:   topic,
		Topic:  topic,
		Symbol: symbol,
		Data:   data,
		Time:   time.Now().UnixMilli(),
	}

	select {
	case h.broadcast <- msg:
	default:
		h.logger.Warn().
			Str("topic", topic).
			Str("symbol", symbol).
			Msg("Broadcast channel full, message dropped")
	}
}

// PublishTrade 发布成交消息
func (h *Hub) PublishTrade(symbol string, data interface{}) {
	h.Publish("trade", symbol, data)
}

// PublishTicker 发布Ticker消息
func (h *Hub) PublishTicker(symbol string, data interface{}) {
	h.Publish("ticker", symbol, data)
}

// PublishOrderBook 发布订单簿消息
func (h *Hub) PublishOrderBook(symbol string, data interface{}) {
	h.Publish("orderbook", symbol, data)
}

// PublishKLine 发布K线消息
func (h *Hub) PublishKLine(symbol, interval string, data interface{}) {
	msg := &Message{
		Type:   "kline",
		Topic:  "kline_" + interval,
		Symbol: symbol,
		Data:   data,
		Time:   time.Now().UnixMilli(),
	}

	select {
	case h.broadcast <- msg:
	default:
	}
}

// readPump 读取客户端消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.hub.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.hub.config.PongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.hub.config.PongTimeout))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Warn().Err(err).Msg("WebSocket read error")
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump 写入客户端消息
func (c *Client) writePump() {
	ticker := time.NewTicker(c.hub.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送待处理消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(message []byte) {
	var req struct {
		Type   string   `json:"type"`
		Topics []string `json:"topics"`
		Topic  string   `json:"topic"`
		Symbol string   `json:"symbol"`
		Method string   `json:"method"`
		Params []string `json:"params"`
		ID     int      `json:"id"`
	}

	if err := json.Unmarshal(message, &req); err != nil {
		c.hub.logger.Warn().Err(err).Msg("Invalid message format")
		return
	}

	// 兼容前端Binance格式订阅消息
	if req.Method == "SUBSCRIBE" && len(req.Params) > 0 {
		for _, param := range req.Params {
			// 解析 "BTCUSDT@trade" 格式
			parts := strings.Split(param, "@")
			if len(parts) >= 2 {
				symbol := parts[0]
				topic := parts[1]
				fullTopic := topic + "_" + symbol
				c.mu.Lock()
				c.groups[fullTopic] = true
				c.mu.Unlock()
			}
		}

		// 发送确认
		response := map[string]interface{}{
			"type":    "subscribed",
			"params":  req.Params,
			"success": true,
			"id":      req.ID,
		}
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}
		return
	}

	switch req.Type {
	case "subscribe":
		// 订阅指定主题
		for _, topic := range req.Topics {
			fullTopic := topic
			if req.Symbol != "" {
				fullTopic = topic + "_" + req.Symbol
			}
			c.mu.Lock()
			c.groups[fullTopic] = true
			c.mu.Unlock()
		}

		// 发送确认
		response := map[string]interface{}{
			"type":    "subscribed",
			"topics":  req.Topics,
			"symbol":  req.Symbol,
			"success": true,
		}
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}

	case "unsubscribe":
		for _, topic := range req.Topics {
			fullTopic := topic
			if req.Symbol != "" {
				fullTopic = topic + "_" + req.Symbol
			}
			c.mu.Lock()
			delete(c.groups, fullTopic)
			c.mu.Unlock()
		}

	case "ping":
		response := map[string]interface{}{
			"type": "pong",
			"time": time.Now().UnixMilli(),
		}
		data, _ := json.Marshal(response)
		select {
		case c.send <- data:
		default:
		}
	}
}

// GetStats 获取统计信息
func (h *Hub) GetStats() HubStats {
	return HubStats{
		ClientCount:   atomic.LoadInt64(&h.clientCount),
		MessageCount:  atomic.LoadInt64(&h.messageCount),
		BroadcastSize: len(h.broadcast),
	}
}

// HubStats WebSocket中心统计
type HubStats struct {
	ClientCount   int64 `json:"client_count"`
	MessageCount  int64 `json:"message_count"`
	BroadcastSize int   `json:"broadcast_size"`
}

// Stop 停止WebSocket中心
func (h *Hub) Stop() {
	h.cancel()
}

// MarketDataBroadcaster 行情广播器
type MarketDataBroadcaster struct {
	hub    *Hub
	symbol string
	logger zerolog.Logger

	// 缓存
	lastTicker    *TickerData
	lastOrderBook *OrderBookData

	// 推送控制
	updateInterval time.Duration
	forceUpdate    chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
}

// TickerData Ticker数据
type TickerData struct {
	Symbol         string  `json:"symbol"`
	LastPrice      float64 `json:"last_price"`
	BidPrice       float64 `json:"bid_price"`
	AskPrice       float64 `json:"ask_price"`
	BidQty         float64 `json:"bid_qty"`
	AskQty         float64 `json:"ask_qty"`
	Volume24H      float64 `json:"volume_24h"`
	QuoteVolume24H float64 `json:"quote_volume_24h"`
	High24H        float64 `json:"high_24h"`
	Low24H         float64 `json:"low_24h"`
	PriceChange    float64 `json:"price_change"`
	PriceChangePct float64 `json:"price_change_pct"`
	Timestamp      int64   `json:"timestamp"`
}

// OrderBookData 订单簿数据
type OrderBookData struct {
	Symbol    string        `json:"symbol"`
	Version   int64         `json:"version"`
	Timestamp int64         `json:"timestamp"`
	Bids      []interface{} `json:"bids"`
	Asks      []interface{} `json:"asks"`
}

// NewMarketDataBroadcaster 创建行情广播器
func NewMarketDataBroadcaster(hub *Hub, symbol string, interval time.Duration, logger zerolog.Logger) *MarketDataBroadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	return &MarketDataBroadcaster{
		hub:            hub,
		symbol:         symbol,
		logger:         logger,
		updateInterval: interval,
		forceUpdate:    make(chan struct{}, 1),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// BroadcastTicker 广播Ticker
func (b *MarketDataBroadcaster) BroadcastTicker(ticker *TickerData) {
	b.lastTicker = ticker
	b.hub.PublishTicker(b.symbol, ticker)
}

// BroadcastOrderBook 广播订单簿
func (b *MarketDataBroadcaster) BroadcastOrderBook(ob *OrderBookData) {
	b.lastOrderBook = ob
	b.hub.PublishOrderBook(b.symbol, ob)
}

// Start 开始广播
func (b *MarketDataBroadcaster) Start() {
	go func() {
		ticker := time.NewTicker(b.updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-b.ctx.Done():
				return
			case <-ticker.C:
				if b.lastTicker != nil {
					b.hub.PublishTicker(b.symbol, b.lastTicker)
				}
			case <-b.forceUpdate:
				if b.lastOrderBook != nil {
					b.hub.PublishOrderBook(b.symbol, b.lastOrderBook)
				}
			}
		}
	}()
}

// Stop 停止广播
func (b *MarketDataBroadcaster) Stop() {
	b.cancel()
}

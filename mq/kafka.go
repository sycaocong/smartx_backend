package mq

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// KafkaTopics Kafka主题配置
type KafkaTopics struct {
	Orders    string
	Trades    string
	Ticker    string
	OrderBook string
}

// KafkaProducer Kafka生产者接口
type KafkaProducer interface {
	SendOrder(order map[string]interface{}) error
	SendTrade(trade map[string]interface{}) error
	SendTicker(ticker map[string]interface{}) error
	SendOrderBook(orderbook map[string]interface{}) error
	Close() error
}

// KafkaConsumer Kafka消费者接口
type KafkaConsumer interface {
	RegisterHandler(topic string, handler MessageHandler)
	Start(topics []string) error
	Stop() error
}

// MessageHandler 消息处理器
type MessageHandler func(topic string, partition int32, data []byte) error

// KafkaMessage Kafka消息结构
type KafkaMessage struct {
	Type      string      `json:"type"`
	Symbol    string      `json:"symbol"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// MockProducer 模拟Kafka生产者（用于测试和开发）
type MockProducer struct {
	topics KafkaTopics
	logger zerolog.Logger
	mu     sync.RWMutex
	sent   []map[string]interface{}
}

// NewMockProducer 创建模拟生产者
func NewMockProducer(topics KafkaTopics, logger zerolog.Logger) *MockProducer {
	return &MockProducer{
		topics: topics,
		logger: logger,
		sent:   make([]map[string]interface{}, 0),
	}
}

// SendOrder 发送订单消息
func (p *MockProducer) SendOrder(order map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data, _ := json.Marshal(order)
	p.logger.Debug().
		Str("topic", p.topics.Orders).
		Str("data", string(data)).
		Msg("Mock: Order message sent")
	
	p.sent = append(p.sent, order)
	return nil
}

// SendTrade 发送成交消息
func (p *MockProducer) SendTrade(trade map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data, _ := json.Marshal(trade)
	p.logger.Debug().
		Str("topic", p.topics.Trades).
		Str("data", string(data)).
		Msg("Mock: Trade message sent")
	
	p.sent = append(p.sent, trade)
	return nil
}

// SendTicker 发送Ticker消息
func (p *MockProducer) SendTicker(ticker map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data, _ := json.Marshal(ticker)
	p.logger.Debug().
		Str("topic", p.topics.Ticker).
		Str("data", string(data)).
		Msg("Mock: Ticker message sent")
	
	p.sent = append(p.sent, ticker)
	return nil
}

// SendOrderBook 发送订单簿消息
func (p *MockProducer) SendOrderBook(orderbook map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data, _ := json.Marshal(orderbook)
	p.logger.Debug().
		Str("topic", p.topics.OrderBook).
		Str("data", string(data)).
		Msg("Mock: OrderBook message sent")
	
	p.sent = append(p.sent, orderbook)
	return nil
}

// Close 关闭
func (p *MockProducer) Close() error {
	p.logger.Info().Msg("Mock: Kafka producer closed")
	return nil
}

// GetSent 获取已发送的消息
func (p *MockProducer) GetSent() []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sent
}

// MockConsumer 模拟Kafka消费者
type MockConsumer struct {
	topics   KafkaTopics
	handlers map[string]MessageHandler
	logger  zerolog.Logger
	mu      sync.RWMutex
	running bool
}

// NewMockConsumer 创建模拟消费者
func NewMockConsumer(topics KafkaTopics, logger zerolog.Logger) *MockConsumer {
	return &MockConsumer{
		topics:   topics,
		handlers: make(map[string]MessageHandler),
		logger:   logger,
		running:  false,
	}
}

// RegisterHandler 注册消息处理器
func (c *MockConsumer) RegisterHandler(topic string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[topic] = handler
}

// Start 开始消费
func (c *MockConsumer) Start(topics []string) error {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	
	c.logger.Info().Strs("topics", topics).Msg("Mock: Consumer started")
	return nil
}

// Stop 停止消费
func (c *MockConsumer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	c.logger.Info().Msg("Mock: Consumer stopped")
	return nil
}

// NewOrderMessage 创建订单消息
func NewOrderMessage(symbol string, order map[string]interface{}) *KafkaMessage {
	return &KafkaMessage{
		Type:      "order",
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      order,
	}
}

// NewTradeMessage 创建成交消息
func NewTradeMessage(symbol string, trade map[string]interface{}) *KafkaMessage {
	return &KafkaMessage{
		Type:      "trade",
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      trade,
	}
}

// NewTickerMessage 创建Ticker消息
func NewTickerMessage(symbol string, ticker map[string]interface{}) *KafkaMessage {
	return &KafkaMessage{
		Type:      "ticker",
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      ticker,
	}
}

// NewOrderBookMessage 创建订单簿消息
func NewOrderBookMessage(symbol string, orderbook map[string]interface{}) *KafkaMessage {
	return &KafkaMessage{
		Type:      "orderbook",
		Symbol:    symbol,
		Timestamp: time.Now().UnixMilli(),
		Data:      orderbook,
	}
}

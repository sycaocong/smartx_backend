package proto

import (
	"encoding/json"
	"github.com/rs/zerolog"
)

// Serializer 序列化器
type Serializer struct {
	logger zerolog.Logger
}

// NewSerializer 创建序列化器
func NewSerializer(logger zerolog.Logger) *Serializer {
	return &Serializer{
		logger: logger,
	}
}

// Serialize 序列化消息为字节数组
func (s *Serializer) Serialize(msgType string, data interface{}) ([]byte, error) {
	switch msgType {
	case "order":
		return s.serializeOrder(data)
	case "trade":
		return s.serializeTrade(data)
	case "ticker":
		return s.serializeTicker(data)
	case "orderbook":
		return s.serializeOrderBook(data)
	case "kline":
		return s.serializeKLine(data)
	default:
		s.logger.Warn().Str("type", msgType).Msg("Unknown message type")
		return nil, nil
	}
}

// Deserialize 反序列化字节数组为消息
func (s *Serializer) Deserialize(msgType string, data []byte) (interface{}, error) {
	switch msgType {
	case "order":
		return s.deserializeOrder(data)
	case "trade":
		return s.deserializeTrade(data)
	case "ticker":
		return s.deserializeTicker(data)
	case "orderbook":
		return s.deserializeOrderBook(data)
	default:
		s.logger.Warn().Str("type", msgType).Msg("Unknown message type")
		return nil, nil
	}
}

// 序列化方法
func (s *Serializer) serializeOrder(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *Serializer) serializeTrade(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *Serializer) serializeTicker(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *Serializer) serializeOrderBook(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *Serializer) serializeKLine(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// 反序列化方法
func (s *Serializer) deserializeOrder(data []byte) (*OrderProto, error) {
	order := &OrderProto{}
	return order, json.Unmarshal(data, order)
}

func (s *Serializer) deserializeTrade(data []byte) (*TradeProto, error) {
	trade := &TradeProto{}
	return trade, json.Unmarshal(data, trade)
}

func (s *Serializer) deserializeTicker(data []byte) (*TickerProto, error) {
	ticker := &TickerProto{}
	return ticker, json.Unmarshal(data, ticker)
}

func (s *Serializer) deserializeOrderBook(data []byte) (*OrderBookProto, error) {
	ob := &OrderBookProto{}
	return ob, json.Unmarshal(data, ob)
}

// CompressionStats 压缩统计
type CompressionStats struct {
	OriginalSize   int
	CompressedSize int
	Ratio          float64
}

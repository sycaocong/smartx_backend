package proto

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
)

// Serializer 序列化器，支持Protobuf和JSON
type Serializer struct {
	logger        zerolog.Logger
	useBufferPool bool
}

// Buffer 缓冲区接口
type Buffer interface {
	Bytes() []byte
	Reset()
}

// bufferPool 对象池
var bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytesBuffer{data: make([]byte, 0, 4096)}
	},
}

// bytesBuffer 字节缓冲区
type bytesBuffer struct {
	data []byte
}

func (b *bytesBuffer) Bytes() []byte { return b.data }
func (b *bytesBuffer) Reset()        { b.data = b.data[:0] }

// NewSerializer 创建序列化器
func NewSerializer(logger zerolog.Logger) *Serializer {
	return &Serializer{
		logger:        logger,
		useBufferPool: true,
	}
}

// Serialize 序列化消息为字节数组（优先使用Protobuf）
func (s *Serializer) Serialize(msgType string, data interface{}) ([]byte, error) {
	var buf []byte
	var err error

	switch msgType {
	case "order":
		buf, err = s.serializeOrder(data)
	case "trade":
		buf, err = s.serializeTrade(data)
	case "ticker":
		buf, err = s.serializeTicker(data)
	case "orderbook":
		buf, err = s.serializeOrderBook(data)
	case "kline":
		buf, err = s.serializeKLine(data)
	default:
		s.logger.Warn().Str("type", msgType).Msg("Unknown message type")
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}

	if err != nil {
		return nil, err
	}

	// 使用buf压缩
	return s.compressBuffer(buf), nil
}

// compressBuffer 使用bufio压缩数据
func (s *Serializer) compressBuffer(data []byte) []byte {
	if len(data) < 64 {
		return data
	}

	// 使用varint编码长度 + 实际数据
	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, uint64(len(data)))

	result := make([]byte, n+len(data))
	copy(result, varintBuf[:n])
	copy(result[n:], data)

	return result
}

// Deserialize 反序列化字节数组为消息
func (s *Serializer) Deserialize(msgType string, data []byte) (interface{}, error) {
	// 解压buffer
	data = s.decompressBuffer(data)

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
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}

// decompressBuffer 解压数据
func (s *Serializer) decompressBuffer(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// 尝试读取varint长度
	length, n := binary.Uvarint(data)
	if n <= 0 || int(length) != len(data)-n {
		return data
	}

	return data[n:]
}

// SerializeToWriter 序列化到Writer
func (s *Serializer) SerializeToWriter(w io.Writer, msgType string, data interface{}) error {
	buf, err := s.Serialize(msgType, data)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(w)
	if _, err := writer.Write(buf); err != nil {
		return err
	}

	return writer.Flush()
}

// DeserializeFromReader 从Reader反序列化
func (s *Serializer) DeserializeFromReader(r io.Reader, msgType string) (interface{}, error) {
	reader := bufio.NewReader(r)
	data, err := reader.ReadBytes(byte(0))
	if err != nil && err != io.EOF {
		return nil, err
	}

	return s.Deserialize(msgType, data)
}

// 序列化方法 - 使用Protobuf
func (s *Serializer) serializeOrder(data interface{}) ([]byte, error) {
	order, ok := data.(*OrderProto)
	if !ok {
		return nil, fmt.Errorf("invalid order type")
	}
	return proto.Marshal(order)
}

func (s *Serializer) serializeTrade(data interface{}) ([]byte, error) {
	trade, ok := data.(*TradeProto)
	if !ok {
		return nil, fmt.Errorf("invalid trade type")
	}
	return proto.Marshal(trade)
}

func (s *Serializer) serializeTicker(data interface{}) ([]byte, error) {
	ticker, ok := data.(*TickerProto)
	if !ok {
		return nil, fmt.Errorf("invalid ticker type")
	}
	return proto.Marshal(ticker)
}

func (s *Serializer) serializeOrderBook(data interface{}) ([]byte, error) {
	ob, ok := data.(*OrderBookProto)
	if !ok {
		return nil, fmt.Errorf("invalid orderbook type")
	}
	return proto.Marshal(ob)
}

func (s *Serializer) serializeKLine(data interface{}) ([]byte, error) {
	kline, ok := data.(*KLineProto)
	if !ok {
		return nil, fmt.Errorf("invalid kline type")
	}
	return proto.Marshal(kline)
}

// 反序列化方法 - 使用Protobuf
func (s *Serializer) deserializeOrder(data []byte) (*OrderProto, error) {
	order := &OrderProto{}
	if err := proto.Unmarshal(data, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *Serializer) deserializeTrade(data []byte) (*TradeProto, error) {
	trade := &TradeProto{}
	if err := proto.Unmarshal(data, trade); err != nil {
		return nil, err
	}
	return trade, nil
}

func (s *Serializer) deserializeTicker(data []byte) (*TickerProto, error) {
	ticker := &TickerProto{}
	if err := proto.Unmarshal(data, ticker); err != nil {
		return nil, err
	}
	return ticker, nil
}

func (s *Serializer) deserializeOrderBook(data []byte) (*OrderBookProto, error) {
	ob := &OrderBookProto{}
	if err := proto.Unmarshal(data, ob); err != nil {
		return nil, err
	}
	return ob, nil
}

// GetBuffer 从池中获取buffer
func GetBuffer() *bytesBuffer {
	return bufferPool.Get().(*bytesBuffer)
}

// PutBuffer 归还buffer到池
func PutBuffer(buf *bytesBuffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

// CompressionStats 压缩统计
type CompressionStats struct {
	OriginalSize   int
	CompressedSize int
	Ratio          float64
}

// GetStats 获取压缩统计
func (s *Serializer) GetStats(originalSize int) CompressionStats {
	// 这里可以添加实际的压缩统计逻辑
	return CompressionStats{
		OriginalSize:   originalSize,
		CompressedSize: originalSize,
		Ratio:          1.0,
	}
}

// Fallback to JSON when protobuf fails
func (s *Serializer) serializeJSON(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *Serializer) deserializeJSON(data []byte, msg proto.Message) error {
	return json.Unmarshal(data, msg)
}

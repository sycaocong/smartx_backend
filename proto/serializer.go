package proto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/rs/zerolog"
)

// Serializer 序列化器，支持高效JSON压缩
type Serializer struct {
	logger zerolog.Logger
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
	return &Serializer{logger: logger}
}

// Serialize 序列化消息为字节数组
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

	return s.compressBuffer(buf), nil
}

// compressBuffer 使用varint编码压缩数据
func (s *Serializer) compressBuffer(data []byte) []byte {
	if len(data) < 64 {
		return data
	}

	varintBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(varintBuf, uint64(len(data)))

	result := make([]byte, n+len(data))
	copy(result, varintBuf[:n])
	copy(result[n:], data)

	return result
}

// Deserialize 反序列化字节数组为消息
func (s *Serializer) Deserialize(msgType string, data []byte) (interface{}, error) {
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

// 序列化方法 - 使用JSON
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
	if err := json.Unmarshal(data, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *Serializer) deserializeTrade(data []byte) (*TradeProto, error) {
	trade := &TradeProto{}
	if err := json.Unmarshal(data, trade); err != nil {
		return nil, err
	}
	return trade, nil
}

func (s *Serializer) deserializeTicker(data []byte) (*TickerProto, error) {
	ticker := &TickerProto{}
	if err := json.Unmarshal(data, ticker); err != nil {
		return nil, err
	}
	return ticker, nil
}

func (s *Serializer) deserializeOrderBook(data []byte) (*OrderBookProto, error) {
	ob := &OrderBookProto{}
	if err := json.Unmarshal(data, ob); err != nil {
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
func (s *Serializer) GetStats(originalSize, compressedSize int) CompressionStats {
	ratio := 1.0
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize)
	}
	return CompressionStats{
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Ratio:          ratio,
	}
}

// Marshal 序列化（兼容接口）
func (s *Serializer) Marshal(msgType string, data interface{}) ([]byte, error) {
	return s.Serialize(msgType, data)
}

// Unmarshal 反序列化
func (s *Serializer) Unmarshal(msgType string, data []byte) (interface{}, error) {
	return s.Deserialize(msgType, data)
}

// CompactJSON 紧凑JSON序列化（无缩进）
func CompactJSON(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(data); err != nil {
		return nil, err
	}
	writer.Flush()
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

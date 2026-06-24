package config

import (
	"os"
	"strconv"
	"sync"
)

type Config struct {
	Server   ServerConfig
	Kafka    KafkaConfig
	Redis    RedisConfig
	Matching MatchingConfig
}

type ServerConfig struct {
	Host string
	Port int
	WS   WSConfig
}

type WSConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	MaxMessageSize  int64
	PingInterval    int
	PongTimeout     int
}

type KafkaConfig struct {
	Brokers []string
	Topic   KafkaTopicConfig
	Producer ProducerConfig
	Consumer ConsumerConfig
}

type KafkaTopicConfig struct {
	Orders       string
	Trades       string
	Ticker       string
	OrderBook    string
}

type ProducerConfig struct {
	Acks         string
	Retries      int
	BatchSize    int
	LingerMs     int
	Compression  string
}

type ConsumerConfig struct {
	GroupID      string
	MinBytes     int
	MaxBytes     int
	MaxWaitMs    int
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type MatchingConfig struct {
	Shards         int
	WorkerPoolSize int
	BufferSize     int
}

var (
	cfg  *Config
	once sync.Once
)

func Load() *Config {
	once.Do(func() {
		cfg = &Config{
			Server: ServerConfig{
				Host: getEnv("SERVER_HOST", "0.0.0.0"),
				Port: getEnvInt("SERVER_PORT", 8080),
				WS: WSConfig{
					ReadBufferSize:  1024,
					WriteBufferSize: 1024,
					MaxMessageSize:  65536,
					PingInterval:    30,
					PongTimeout:     60,
				},
			},
			Kafka: KafkaConfig{
				Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
				Topic: KafkaTopicConfig{
					Orders:    getEnv("KAFKA_TOPIC_ORDERS", "exchange.orders"),
					Trades:    getEnv("KAFKA_TOPIC_TRADES", "exchange.trades"),
					Ticker:    getEnv("KAFKA_TOPIC_TICKER", "exchange.ticker"),
					OrderBook: getEnv("KAFKA_TOPIC_ORDERBOOK", "exchange.orderbook"),
				},
				Producer: ProducerConfig{
					Acks:        getEnv("KAFKA_PRODUCER_ACKS", "all"),
					Retries:     getEnvInt("KAFKA_PRODUCER_RETRIES", 3),
					BatchSize:   getEnvInt("KAFKA_PRODUCER_BATCH_SIZE", 16384),
					LingerMs:    getEnvInt("KAFKA_PRODUCER_LINGER_MS", 5),
					Compression: getEnv("KAFKA_PRODUCER_COMPRESSION", "lz4"),
				},
				Consumer: ConsumerConfig{
					GroupID:   getEnv("KAFKA_CONSUMER_GROUP_ID", "matching-engine"),
					MinBytes:  getEnvInt("KAFKA_CONSUMER_MIN_BYTES", 1),
					MaxBytes:  getEnvInt("KAFKA_CONSUMER_MAX_BYTES", 10485760),
					MaxWaitMs: getEnvInt("KAFKA_CONSUMER_MAX_WAIT_MS", 500),
				},
			},
			Redis: RedisConfig{
				Host:     getEnv("REDIS_HOST", "localhost"),
				Port:     getEnvInt("REDIS_PORT", 6379),
				Password: getEnv("REDIS_PASSWORD", ""),
				DB:       getEnvInt("REDIS_DB", 0),
			},
			Matching: MatchingConfig{
				Shards:         getEnvInt("MATCHING_SHARDS", 8),
				WorkerPoolSize: getEnvInt("MATCHING_WORKER_POOL_SIZE", 16),
				BufferSize:     getEnvInt("MATCHING_BUFFER_SIZE", 10000),
			},
		}
	})
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

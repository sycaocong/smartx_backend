package proto

// OrderProto 订单protobuf结构
type OrderProto struct {
	OrderId    string  `json:"order_id"`
	Symbol     string  `json:"symbol"`
	Side       int32   `json:"side"`
	Type       int32   `json:"type"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	FilledQty  float64 `json:"filled_qty"`
	AvgPrice   float64 `json:"avg_price"`
	Status     int32   `json:"status"`
	Timestamp  int64   `json:"timestamp"`
	ClientOid  string  `json:"client_oid"`
}

// TradeProto 成交protobuf结构
type TradeProto struct {
	TradeId    string  `json:"trade_id"`
	Symbol     string  `json:"symbol"`
	OrderId    string  `json:"order_id"`
	CounterOid string  `json:"counter_oid"`
	Side       int32   `json:"side"`
	Price      float64 `json:"price"`
	Quantity   float64 `json:"quantity"`
	Fee        float64 `json:"fee"`
	Timestamp  int64   `json:"timestamp"`
}

// TickerProto Ticker protobuf结构
type TickerProto struct {
	Symbol         string  `json:"symbol"`
	LastPrice      float64 `json:"last_price"`
	BidPrice       float64 `json:"bid_price"`
	AskPrice       float64 `json:"ask_price"`
	BidQty         float64 `json:"bid_qty"`
	AskQty         float64 `json:"ask_qty"`
	Volume24H      float64 `json:"volume_24h"`
	QuoteVolume24H float64 `json:"quote_volume_24h"`
	Timestamp      int64   `json:"timestamp"`
}

// OrderBookProto 订单簿protobuf结构
type OrderBookProto struct {
	Symbol    string    `json:"symbol"`
	Version   int64     `json:"version"`
	Timestamp int64     `json:"timestamp"`
	Bids      []float64 `json:"bids"` // [price1, qty1, price2, qty2, ...]
	Asks      []float64 `json:"asks"`
}

// KLineProto K线protobuf结构
type KLineProto struct {
	Symbol    string  `json:"symbol"`
	Interval  string  `json:"interval"`
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

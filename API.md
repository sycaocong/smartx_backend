# SmartX 撮合引擎 API 接口文档

## 1. 概述

- **项目名称**: SmartX 企业级高性能交易所撮合引擎
- **基础URL**: `http://localhost:8080`
- **WebSocket**: `ws://localhost:8080/ws`
- **数据格式**: JSON
- **字符编码**: UTF-8
- **开发语言**: Go 1.21+

---

## 2. HTTP API

### 2.1 健康检查

**GET** `/health`

检查服务健康状态。

**请求参数**: 无

**响应示例**:
```json
{
    "status": "healthy"
}
```

**代码位置**: [handler.go#L129](api/handler.go#L129)

---

### 2.2 就绪检查

**GET** `/ready`

检查服务就绪状态。

**请求参数**: 无

**响应示例**:
```json
{
    "status": "ready"
}
```

**代码位置**: [handler.go#L134](api/handler.go#L134)

---

### 2.3 获取行情 Ticker

**GET** `/api/v1/market/ticker/{symbol}`

获取指定交易对的最新行情数据。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**响应示例**:
```json
{
    "symbol": "BTCUSDT",
    "last_price": 94250.50,
    "bid_price": 94250.00,
    "ask_price": 94251.00,
    "bid_qty": 1.2345,
    "ask_qty": 0.9876,
    "volume_24h": 12345.67,
    "quote_volume_24h": 1167890123.45,
    "high_24h": 95500.00,
    "low_24h": 92800.00,
    "price_change": 1250.30,
    "price_change_pct": 1.34,
    "timestamp": 1750828800000
}
```

**响应字段**:
| 字段 | 类型 | 说明 |
|------|------|------|
| symbol | string | 交易对 |
| last_price | float64 | 最新成交价 |
| bid_price | float64 | 买一价 |
| ask_price | float64 | 卖一价 |
| bid_qty | float64 | 买一量 |
| ask_qty | float64 | 卖一量 |
| volume_24h | float64 | 24小时成交量 |
| quote_volume_24h | float64 | 24小时成交额 |
| high_24h | float64 | 24小时最高价 |
| low_24h | float64 | 24小时最低价 |
| price_change | float64 | 24小时价格变动 |
| price_change_pct | float64 | 24小时涨跌幅(%) |
| timestamp | int64 | 时间戳(毫秒) |

**代码位置**: [handler.go#L256](api/handler.go#L256)

---

### 2.4 获取订单簿深度

**GET** `/api/v1/market/orderbook/{symbol}`

获取指定交易对的订单簿深度。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| limit | int | 否 | 20 | 深度档位数量 |

**响应示例**:
```json
{
    "symbol": "BTCUSDT",
    "version": 1750828800001,
    "timestamp": 1750828800000,
    "bids": [
        [94250.50, 1.2345],
        [94249.00, 2.3456],
        [94248.50, 0.5678]
    ],
    "asks": [
        [94251.00, 0.9876],
        [94252.50, 1.5432],
        [94253.00, 2.1111]
    ]
}
```

**响应字段**:
| 字段 | 类型 | 说明 |
|------|------|------|
| symbol | string | 交易对 |
| version | int64 | 版本号(纳秒时间戳) |
| timestamp | int64 | 时间戳(毫秒) |
| bids | array | 买单列表，每项为 [价格, 数量] |
| asks | array | 卖单列表，每项为 [价格, 数量] |

**代码位置**: [handler.go#L283](api/handler.go#L283)

---

### 2.5 获取深度

**GET** `/api/v1/market/depth/{symbol}`

获取指定交易对的深度数据（原始格式）。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| limit | int | 否 | 20 | 深度档位数量 |

**响应示例**:
```json
{
    "symbol": "BTCUSDT",
    "bids": [
        {"price": 94250.50, "quantity": 1.2345},
        {"price": 94249.00, "quantity": 2.3456}
    ],
    "asks": [
        {"price": 94251.00, "quantity": 0.9876},
        {"price": 94252.50, "quantity": 1.5432}
    ]
}
```

**代码位置**: [handler.go#L355](api/handler.go#L355)

---

### 2.6 获取成交历史

**GET** `/api/v1/market/trades/{symbol}`

获取指定交易对的成交历史。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**响应示例**:
```json
{
    "trades": [],
    "symbol": "BTCUSDT"
}
```

> **注意**: 当前为占位实现，返回空数组。

**代码位置**: [handler.go#L319](api/handler.go#L319)

---

### 2.7 获取K线数据

**GET** `/api/v1/market/kline/{symbol}`

获取指定交易对的K线数据。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| interval | string | 否 | 1m | K线周期(1m, 5m, 1h, 1d) |
| limit | int | 否 | 100 | 返回数量 |

**响应示例**:
```json
{
    "symbol": "BTCUSDT",
    "interval": "1m",
    "klines": [],
    "limit": 100
}
```

> **注意**: 当前为占位实现，返回空数组。

**代码位置**: [handler.go#L330](api/handler.go#L330)

---

### 2.8 创建订单

**POST** `/api/v1/orders`

创建新订单。

**请求头**:
```
Content-Type: application/json
```

**请求体**:
```json
{
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": 94250.50,
    "quantity": 0.1234,
    "client_order_id": "my-order-123"
}
```

**请求字段**:
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对 |
| side | string | 是 | BUY 或 SELL |
| type | string | 是 | LIMIT 或 MARKET |
| price | float64 | 否* | 订单价格（市价单可省略） |
| quantity | float64 | 是 | 订单数量（必须 > 0） |
| client_order_id | string | 否 | 客户端订单ID |

**响应示例**:
```json
{
    "order_id": "a0a4a2e4-e598-4f36-88cf-43af22304f45",
    "symbol": "BTCUSDT",
    "side": 0,
    "type": 0,
    "price": 94250.50,
    "quantity": 0.1234,
    "filled_quantity": 0,
    "avg_fill_price": 0,
    "status": 0,
    "timestamp": 1750828800000,
    "client_order_id": "my-order-123"
}
```

**响应字段**:
| 字段 | 类型 | 说明 |
|------|------|------|
| order_id | string | 订单ID |
| symbol | string | 交易对 |
| side | int | 方向(0=BUY, 1=SELL) |
| type | int | 类型(0=LIMIT, 1=MARKET) |
| price | float64 | 订单价格 |
| quantity | float64 | 订单数量 |
| filled_quantity | float64 | 已成交数量 |
| avg_fill_price | float64 | 平均成交价 |
| status | int | 状态(0=NEW, 1=PARTIALLY_FILLED, 2=FILLED, 3=CANCELED, 4=REJECTED) |
| timestamp | int64 | 创建时间戳 |
| client_order_id | string | 客户端订单ID |

**代码位置**: [handler.go#L174](api/handler.go#L174)

---

### 2.9 查询订单

**GET** `/api/v1/orders/{orderId}`

查询指定订单详情。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| orderId | string | 是 | 订单ID |

**响应示例**:
```json
{
    "order_id": "a0a4a2e4-e598-4f36-88cf-43af22304f45",
    "status": "not_implemented"
}
```

> **注意**: 当前为占位实现，未查询真实订单数据。

**代码位置**: [handler.go#L221](api/handler.go#L221)

---

### 2.10 取消订单

**DELETE** `/api/v1/orders/{orderId}`

取消指定订单。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| orderId | string | 是 | 订单ID |

**响应示例**:
```json
{
    "order_id": "a0a4a2e4-e598-4f36-88cf-43af22304f45",
    "status": "canceled"
}
```

> **注意**: 当前为占位实现，未执行真实取消操作。

**代码位置**: [handler.go#L230](api/handler.go#L230)

---

### 2.11 获取订单列表

**GET** `/api/v1/orders`

获取订单列表。

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| symbol | string | 否 | - | 交易对过滤 |
| limit | string | 否 | 100 | 返回数量 |

**响应示例**:
```json
{
    "orders": [],
    "limit": "100",
    "symbol": "BTCUSDT"
}
```

> **注意**: 当前为占位实现，返回空数组。

**代码位置**: [handler.go#L239](api/handler.go#L239)

---

### 2.12 获取统计信息

**GET** `/api/v1/stats`

获取系统统计信息。

**响应示例**:
```json
{
    "ws_clients": 15,
    "ws_messages": 123456,
    "timestamp": 1750828800000
}
```

**响应字段**:
| 字段 | 类型 | 说明 |
|------|------|------|
| ws_clients | int64 | WebSocket客户端数量 |
| ws_messages | int64 | 已发送消息总数 |
| timestamp | int64 | 时间戳(毫秒) |

**代码位置**: [handler.go#L382](api/handler.go#L382)

---

### 2.13 获取分片统计

**GET** `/api/v1/stats/shard`

获取分片统计信息。

**响应示例**:
```json
{
    "shards": []
}
```

> **注意**: 当前为占位实现，返回空数组。

**代码位置**: [handler.go#L394](api/handler.go#L394)

---

## 3. WebSocket API

### 3.1 连接

**URL**: `ws://localhost:8080/ws`

**连接示例**:
```javascript
const ws = new WebSocket('ws://localhost:8080/ws')
```

---

### 3.2 订阅/取消订阅

**订阅消息**（兼容Binance格式）:
```json
{
    "method": "SUBSCRIBE",
    "params": [
        "BTCUSDT@trade",
        "BTCUSDT@depth@100ms"
    ],
    "id": 1
}
```

**取消订阅**:
```json
{
    "method": "UNSUBSCRIBE",
    "params": ["BTCUSDT@trade"],
    "id": 2
}
```

**订阅确认响应**:
```json
{
    "type": "subscribed",
    "params": ["BTCUSDT@trade"],
    "success": true,
    "id": 1
}
```

---

### 3.3 订阅主题

| 主题格式 | 说明 | 示例 |
|---------|------|------|
| `<symbol>@trade` | 实时成交 | BTCUSDT@trade |
| `<symbol>@depth@<interval>` | 订单簿深度更新 | BTCUSDT@depth@100ms |
| `<symbol>@kline_<interval>` | K线数据 | BTCUSDT@kline_1m |

**代码位置**: [hub.go#L341](ws/hub.go#L341)

---

### 3.4 推送消息格式

**推送消息通用结构**:
```json
{
    "type": "trade",
    "topic": "trade_BTCUSDT",
    "symbol": "BTCUSDT",
    "data": {},
    "time": 1750828800000
}
```

**成交消息 (trade)**:
```json
{
    "type": "trade",
    "topic": "trade_BTCUSDT",
    "symbol": "BTCUSDT",
    "data": {
        "trade_id": "trd-abc123",
        "order_id": "ord-xyz789",
        "side": 0,
        "price": 94250.50,
        "quantity": 0.1234,
        "timestamp": 1750828800000
    },
    "time": 1750828800000
}
```

**订单簿消息 (orderbook)**:
```json
{
    "type": "orderbook",
    "topic": "orderbook_BTCUSDT",
    "symbol": "BTCUSDT",
    "data": {
        "symbol": "BTCUSDT",
        "version": 1750828800001,
        "timestamp": 1750828800000,
        "bids": [[94250.50, 1.2345], [94249.00, 2.3456]],
        "asks": [[94251.00, 0.9876], [94252.50, 1.5432]]
    },
    "time": 1750828800000
}
```

**Ticker消息**:
```json
{
    "type": "ticker",
    "topic": "ticker_BTCUSDT",
    "symbol": "BTCUSDT",
    "data": {
        "symbol": "BTCUSDT",
        "last_price": 94250.50,
        "bid_price": 94250.00,
        "ask_price": 94251.00,
        "volume_24h": 12345.67,
        "timestamp": 1750828800000
    },
    "time": 1750828800000
}
```

---

### 3.5 心跳机制

**Ping请求**:
```json
{
    "type": "ping"
}
```

**Pong响应**:
```json
{
    "type": "pong",
    "time": 1750828800000
}
```

**自动心跳**: 服务端每30秒发送Ping消息，客户端需在30秒内回复Pong，否则连接会被断开。

**代码位置**: [hub.go#L298](ws/hub.go#L298)

---

## 4. 数据类型

### 4.1 订单方向 (OrderSide)

| 值 | 枚举 | 说明 |
|----|------|------|
| 0 | Buy | 买入 |
| 1 | Sell | 卖出 |

**代码位置**: [order.go#L10](engine/order.go#L10)

---

### 4.2 订单类型 (OrderType)

| 值 | 枚举 | 说明 |
|----|------|------|
| 0 | LimitOrder | 限价单 |
| 1 | MarketOrder | 市价单 |
| 2 | StopLimitOrder | 止限价单 |

**代码位置**: [order.go#L18](engine/order.go#L18)

---

### 4.3 订单状态 (OrderStatus)

| 值 | 枚举 | 说明 |
|----|------|------|
| 0 | StatusNew | 新建 |
| 1 | StatusPartiallyFilled | 部分成交 |
| 2 | StatusFilled | 完全成交 |
| 3 | StatusCanceled | 已取消 |
| 4 | StatusRejected | 已拒绝 |

**代码位置**: [order.go#L27](engine/order.go#L27)

---

### 4.4 订单结构 (Order)

```go
type Order struct {
    OrderID        string        // 订单ID
    Symbol         string        // 交易对
    Side           OrderSide     // 买卖方向(0=Buy, 1=Sell)
    Type           OrderType     // 订单类型(0=LIMIT, 1=MARKET)
    Price          float64       // 价格
    Quantity       float64       // 数量
    FilledQuantity float64       // 已成交数量
    AvgFillPrice   float64       // 平均成交价
    Status         OrderStatus   // 状态
    Timestamp      int64         // 时间戳(毫秒)
    ClientOrderID  string        // 客户端订单ID
    Priority       int64         // 优先级(纳秒时间戳)
}
```

**代码位置**: [order.go#L38](engine/order.go#L38)

---

### 4.5 成交记录结构 (Trade)

```go
type Trade struct {
    TradeID        string    // 成交ID
    Symbol         string    // 交易对
    OrderID        string    // 订单ID
    CounterOrderID string    // 对手订单ID
    Side           OrderSide // 方向
    Price          float64   // 成交价
    Quantity       float64   // 成交数量
    Fee            float64   // 手续费
    FeeCurrency    string    // 手续费货币
    Timestamp      int64     // 时间戳(毫秒)
}
```

**代码位置**: [order.go#L184](engine/order.go#L184)

---

### 4.6 Ticker数据结构 (TickerData)

```go
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
```

**代码位置**: [hub.go#L477](ws/hub.go#L477)

---

### 4.7 订单簿数据结构 (OrderBookData)

```go
type OrderBookData struct {
    Symbol    string        `json:"symbol"`
    Version   int64         `json:"version"`
    Timestamp int64         `json:"timestamp"`
    Bids      []interface{} `json:"bids"` // [[price, quantity], ...]
    Asks      []interface{} `json:"asks"` // [[price, quantity], ...]
}
```

**代码位置**: [hub.go#L494](ws/hub.go#L494)

---

## 5. 错误响应

### 5.1 错误响应格式

```json
{
    "success": false,
    "error": {
        "code": "INVALID_REQUEST",
        "message": "请求参数无效"
    }
}
```

### 5.2 HTTP错误码

| HTTP状态码 | 说明 |
|-----------|------|
| 400 | 请求参数无效 |
| 404 | 资源未找到 |
| 405 | 方法不允许 |
| 500 | 服务器内部错误 |

### 5.3 常见错误场景

| 错误场景 | HTTP状态码 | 响应内容 |
|---------|-----------|---------|
| 请求体解析失败 | 400 | `"Invalid request body"` |
| 必填字段缺失 | 400 | `"Missing required fields"` |
| 交易对不存在 | 404 | `"Symbol not found"` |
| 请求方法错误 | 405 | `"Method not allowed"` |
| 撮合引擎错误 | 500 | 具体错误信息 |

**代码位置**: [handler.go#L402](api/handler.go#L402)

---

## 6. 限流规则

| 接口 | 限制 |
|------|------|
| HTTP API | 1000 请求/分钟 |
| WebSocket | 100 订阅/连接 |
| 下单 | 100 订单/秒 |

---

## 7. 路由映射表

| HTTP方法 | 路径 | 处理函数 | 文件位置 |
|---------|------|---------|---------|
| GET | /health | Health | api/handler.go:129 |
| GET | /ready | Ready | api/handler.go:134 |
| GET | /ws | WebSocket | api/handler.go:140 |
| POST | /api/v1/orders | CreateOrder | api/handler.go:174 |
| GET | /api/v1/orders | GetOrders | api/handler.go:239 |
| GET | /api/v1/orders/{orderId} | GetOrder | api/handler.go:221 |
| DELETE | /api/v1/orders/{orderId} | CancelOrder | api/handler.go:230 |
| GET | /api/v1/market/ticker/{symbol} | GetTicker | api/handler.go:256 |
| GET | /api/v1/market/orderbook/{symbol} | GetOrderBook | api/handler.go:283 |
| GET | /api/v1/market/trades/{symbol} | GetTrades | api/handler.go:319 |
| GET | /api/v1/market/kline/{symbol} | GetKLine | api/handler.go:330 |
| GET | /api/v1/market/depth/{symbol} | GetDepth | api/handler.go:355 |
| GET | /api/v1/stats | GetStats | api/handler.go:382 |
| GET | /api/v1/stats/shard | GetShardStats | api/handler.go:394 |

**路由注册位置**: [handler.go#L48](api/handler.go#L48)

---

## 8. 示例代码

### 8.1 Go 下单示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    order := map[string]interface{}{
        "symbol":   "BTCUSDT",
        "side":     "BUY",
        "type":     "LIMIT",
        "price":    94250.50,
        "quantity": 0.1234,
    }
    
    jsonData, _ := json.Marshal(order)
    resp, _ := http.Post("http://localhost:8080/api/v1/orders", 
        "application/json", bytes.NewBuffer(jsonData))
    defer resp.Body.Close()
    
    fmt.Println("Status:", resp.Status)
}
```

### 8.2 JavaScript WebSocket 订阅示例

```javascript
const ws = new WebSocket('ws://localhost:8080/ws')

ws.onopen = () => {
    ws.send(JSON.stringify({
        method: 'SUBSCRIBE',
        params: ['BTCUSDT@trade', 'BTCUSDT@depth@100ms'],
        id: 1
    }))
}

ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    console.log('Received:', data)
}

ws.onerror = (error) => {
    console.error('WebSocket error:', error)
}
```

### 8.3 PowerShell 测试示例

```powershell
# 健康检查
Invoke-RestMethod -Uri "http://localhost:8080/health"

# 获取Ticker
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/market/ticker/BTCUSDT"

# 创建订单
$body = @{
    symbol = "BTCUSDT"
    side = "BUY"
    type = "LIMIT"
    price = 95000
    quantity = 0.1
} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/orders" -Method Post -ContentType "application/json" -Body $body
```

---

## 9. 待实现功能

| 功能 | 状态 | 说明 |
|------|------|------|
| 查询订单详情 | 占位 | 返回固定响应 |
| 取消订单 | 占位 | 返回固定响应 |
| 获取订单列表 | 占位 | 返回空数组 |
| 获取成交历史 | 占位 | 返回空数组 |
| 获取K线数据 | 占位 | 返回空数组 |
| 获取分片统计 | 占位 | 返回空数组 |
| WebSocket K线推送 | 未实现 | 需开发 |
| 用户认证 | 未实现 | 需添加JWT |
| 钱包余额 | 未实现 | 需开发 |
| 订单撮合后回调 | 未实现 | 需开发 |

---

## 10. 技术栈

| 组件 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.21+ |
| HTTP框架 | 标准库 net/http | - |
| WebSocket | gorilla/websocket | - |
| 日志 | rs/zerolog | - |
| UUID | google/uuid | - |
| 数据结构 | 跳表(SkipList), 红黑树 | 自定义实现 |

---

## 11. 项目结构

```
e:\codex\smartx_backend\
├── api/
│   └── handler.go          # HTTP API处理器
├── cmd/server/
│   └── main.go             # 服务入口
├── config/
│   └── config.go           # 配置管理
├── engine/
│   ├── matching.go         # 撮合逻辑
│   ├── order.go            # 订单结构
│   ├── orderbook.go        # 订单簿
│   ├── shard.go            # 分片路由
│   └── skiplist.go         # 跳表实现
├── mq/
│   └── kafka.go            # Kafka消息队列
├── proto/
│   ├── market.go           # Protobuf定义
│   └── serializer.go       # 序列化工具
├── ws/
│   └── hub.go              # WebSocket中心
├── Dockerfile              # Docker配置
├── docker-compose.yml      # Docker Compose
├── API.md                  # API文档(本文件)
├── DESIGN.md               # 设计文档
├── config.toml             # 配置文件
├── go.mod                  # Go模块依赖
└── go.sum                  # Go依赖校验
```

---

## 12. 部署信息

### 12.1 本地开发

```bash
cd e:\codex\smartx_backend
go run ./cmd/server
```

### 12.2 Docker部署

```bash
cd e:\codex\smartx_backend
docker-compose up -d
```

### 12.3 服务端口

| 端口 | 服务 | 说明 |
|------|------|------|
| 8080 | HTTP/WebSocket | API和WebSocket服务 |
| 9090 | 监控指标 | Prometheus metrics |

---

**文档生成时间**: 2026-06-25
**文档版本**: v1.0
**代码位置**: `e:\codex\smartx_backend`

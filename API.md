# SmartX 撮合引擎 API 接口文档

## 1. 概述

- **基础URL**: `http://localhost:8080`
- **WebSocket**: `ws://localhost:8080/ws`
- **数据格式**: JSON
- **字符编码**: UTF-8

---

## 2. HTTP API

### 2.1 健康检查

**GET** `/health`

检查服务健康状态。

**响应示例**:
```json
{
    "status": "healthy"
}
```

---

### 2.2 获取行情 ticker

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
    "lastPrice": "94250.50",
    "priceChange": "1250.30",
    "priceChangePercent": "1.34",
    "high24h": "95500.00",
    "low24h": "92800.00",
    "volume24h": "12345.67",
    "quoteVolume24h": "1167890123.45",
    "timestamp": 1750828800000
}
```

---

### 2.3 获取订单簿深度

**GET** `/api/v1/market/orderbook/{symbol}`

获取指定交易对的订单簿深度。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对，如 BTCUSDT |

**查询参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| limit | int | 否 | 100 | 深度档位数量 |

**响应示例**:
```json
{
    "symbol": "BTCUSDT",
    "bids": [
        ["94250.50", "1.2345"],
        ["94249.00", "2.3456"],
        ["94248.50", "0.5678"]
    ],
    "asks": [
        ["94251.00", "0.9876"],
        ["94252.50", "1.5432"],
        ["94253.00", "2.1111"]
    ],
    "lastUpdateId": 1750828800001
}
```

---

### 2.4 下单

**POST** `/api/v1/order`

创建新订单。

**请求头**:
```
Content-Type: application/json
X-User-ID: user123 (可选)
```

**请求体**:
```json
{
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": "94250.50",
    "quantity": "0.1234"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol | string | 是 | 交易对 |
| side | string | 是 | BUY 或 SELL |
| type | string | 是 | LIMIT 或 MARKET |
| price | string | 否* | 订单价格 (*市价单可省略) |
| quantity | string | 是 | 订单数量 |

**响应示例**:
```json
{
    "orderId": "ord-550e8400-e29b-41d4-a716-446655440000",
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": "94250.50",
    "quantity": "0.1234",
    "filledQty": "0.0000",
    "status": "OPEN",
    "createTime": 1750828800000,
    "updateTime": 1750828800000
}
```

---

### 2.5 取消订单

**DELETE** `/api/v1/order/{orderId}`

取消指定订单。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| orderId | string | 是 | 订单ID |

**响应示例**:
```json
{
    "orderId": "ord-550e8400-e29b-41d4-a716-446655440000",
    "status": "CANCELED",
    "canceledQty": "0.1234"
}
```

---

### 2.6 查询订单

**GET** `/api/v1/order/{orderId}`

查询指定订单详情。

**路径参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| orderId | string | 是 | 订单ID |

**响应示例**:
```json
{
    "orderId": "ord-550e8400-e29b-41d4-a716-446655440000",
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": "94250.50",
    "quantity": "0.1234",
    "filledQty": "0.0500",
    "status": "PARTIALLY_FILLED",
    "createTime": 1750828800000,
    "updateTime": 1750828800500
}
```

---

### 2.7 错误响应

错误响应格式：

```json
{
    "error": {
        "code": "INVALID_ORDER",
        "message": "订单价格不能为负数",
        "details": {
            "field": "price",
            "value": "-100"
        }
    }
}
```

**错误码列表**:

| 错误码 | HTTP状态码 | 说明 |
|--------|-----------|------|
| INVALID_REQUEST | 400 | 请求参数无效 |
| INVALID_ORDER | 400 | 订单参数无效 |
| ORDER_NOT_FOUND | 404 | 订单不存在 |
| INSUFFICIENT_BALANCE | 400 | 余额不足 |
| MARKET_CLOSED | 400 | 市场已关闭 |
| RATE_LIMITED | 429 | 请求过于频繁 |
| INTERNAL_ERROR | 500 | 服务器内部错误 |

---

## 3. WebSocket API

### 3.1 连接

**URL**: `ws://localhost:8080/ws`

### 3.2 订阅/取消订阅

**订阅消息**:
```json
{
    "method": "SUBSCRIBE",
    "params": [
        "BTCUSDT@trade",
        "BTCUSDT@depth@100ms",
        "BTCUSDT@kline_1m"
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

### 3.3 订阅主题

| 主题 | 参数 | 说明 |
|------|------|------|
| `<symbol>@trade` | 交易对 | 实时成交 |
| `<symbol>@depth@<interval>` | 交易对, 间隔 | 订单簿深度 (100ms, 1s) |
| `<symbol>@kline_<interval>` | 交易对, 间隔 | K线数据 (1m, 5m, 1h, 1d) |
| `<symbol>@ticker` | 交易对 | 行情Ticker |

### 3.4 推送消息

**成交消息 (trade)**:
```json
{
    "e": "trade",
    "s": "BTCUSDT",
    "p": "94250.50",
    "q": "0.1234",
    "m": true,
    "T": 1750828800000
}
```

**深度消息 (depth)**:
```json
{
    "e": "depth",
    "s": "BTCUSDT",
    "bids": [["94250.50", "1.2345"]],
    "asks": [["94251.00", "0.9876"]],
    "u": 1750828800001
}
```

**K线消息 (kline)**:
```json
{
    "e": "kline",
    "s": "BTCUSDT",
    "k": {
        "t": 1750828800000,
        "o": "94000.00",
        "h": "94500.00",
        "l": "93800.00",
        "c": "94250.50",
        "v": "12345.67"
    }
}
```

**Ticker消息**:
```json
{
    "e": "24hrTicker",
    "s": "BTCUSDT",
    "c": "94250.50",
    "p": "1250.30",
    "P": "1.34",
    "h": "95500.00",
    "l": "92800.00",
    "v": "12345.67",
    "q": "1167890123.45"
}
```

---

## 4. 数据类型

### 4.1 订单方向

| 值 | 说明 |
|----|------|
| BUY | 买入 |
| SELL | 卖出 |

### 4.2 订单类型

| 值 | 说明 |
|----|------|
| LIMIT | 限价单 |
| MARKET | 市价单 |

### 4.3 订单状态

| 值 | 说明 |
|----|------|
| PENDING | 等待中 |
| OPEN | 已挂单 |
| PARTIALLY_FILLED | 部分成交 |
| FILLED | 完全成交 |
| CANCELED | 已取消 |
| REJECTED | 已拒绝 |

### 4.4 精度说明

- 价格精度: 8位小数
- 数量精度: 8位小数
- 金额精度: 8位小数
- 时间戳: 毫秒 (Unix Epoch)

---

## 5. 限流规则

| 接口 | 限制 |
|------|------|
| HTTP API | 1000 请求/分钟 |
| WebSocket | 100 订阅/连接 |
| 下单 | 100 订单/秒 |

---

## 6. 示例代码

### 6.1 Go 下单示例

```go
client := &http.Client{}
req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/order", strings.NewReader(`{
    "symbol": "BTCUSDT",
    "side": "BUY",
    "type": "LIMIT",
    "price": "94250.50",
    "quantity": "0.1234"
}`))
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
defer resp.Body.Close()
```

### 6.2 WebSocket 订阅示例 (Go)

```go
conn, _, _ := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)

subscribeMsg := `{"method":"SUBSCRIBE","params":["BTCUSDT@trade"],"id":1}`
conn.WriteMessage(websocket.TextMessage, []byte(subscribeMsg))

for {
    _, msg, _ := conn.ReadMessage()
    fmt.Println(string(msg))
}
```

### 6.3 Python 订阅示例

```python
import websockets
import asyncio
import json

async def main():
    async with websockets.connect("ws://localhost:8080/ws") as ws:
        await ws.send(json.dumps({
            "method": "SUBSCRIBE",
            "params": ["BTCUSDT@trade"],
            "id": 1
        }))
        
        async for message in ws:
            print(message)

asyncio.run(main())
```

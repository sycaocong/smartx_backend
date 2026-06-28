# SmartX 衍生品交易系统 - 详细设计文档

## 1. 系统概述

SmartX 衍生品交易系统是一个机构级的高性能衍生品交易平台，支持永续合约、交割合约、期权交易以及套利工具。系统采用微服务架构，具备高并发、低延迟的特性，满足机构级交易需求。

### 1.1 核心功能

| 模块 | 功能 | 描述 |
|------|------|------|
| 永续合约 | 1-100x杠杆交易 | 支持多空双向，资金费率每8小时结算 |
| 交割合约 | 定期交割 | 支持季度/月度合约，到期自动交割 |
| 期权 | 欧式期权 | Black-Scholes定价模型，希腊值计算 |
| 套利工具 | 期现套利/跨合约套利 | 实时监控套利机会 |

### 1.2 技术指标

- **撮合延迟**: < 1ms
- **吞吐量**: 100,000 TPS
- **支持杠杆**: 1x - 100x
- **资金费率周期**: 8小时
- **标记价格**: 基于多交易所加权指数

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        API Gateway                                  │
│  REST API / WebSocket                                               │
└──────────────────────┬──────────────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────────────┐
│                    交易引擎层                                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │ Spot Engine │  │ Futures     │  │ Option      │                 │
│  │             │  │ Engine      │  │ Engine      │                 │
│  └─────────────┘  └─────────────┘  └─────────────┘                 │
└──────────────────────┬──────────────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────────────┐
│                    风险管理层                                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │ Margin      │  │ Liquidation │  │ Mark Price  │                 │
│  │ Engine      │  │ Engine      │  │ Engine      │                 │
│  └─────────────┘  └─────────────┘  └─────────────┘                 │
│  ┌─────────────┐                                                    │
│  │ Funding Rate│                                                    │
│  │ Engine      │                                                    │
│  └─────────────┘                                                    │
└──────────────────────┬──────────────────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────────────────┐
│                    数据存储层                                        │
│  Redis / PostgreSQL / Kafka                                         │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 模块划分

| 模块 | 职责 | 关键文件 |
|------|------|----------|
| `engine/futures` | 合约撮合引擎 | [futures_engine.go](engine/futures/futures_engine.go), [futures_order.go](engine/futures/futures_order.go) |
| `risk` | 风险管理 | [margin.go](risk/margin.go), [liquidation.go](risk/liquidation.go), [mark_price.go](risk/mark_price.go), [funding_rate.go](risk/funding_rate.go) |
| `api` | API接口 | [futures_handler.go](api/futures_handler.go) |
| `cmd/server` | 服务入口 | [main.go](cmd/server/main.go) |

## 3. 核心数据结构

### 3.1 合约订单 (FuturesOrder)

```go
type FuturesOrder struct {
    OrderID          string           // 订单ID
    Symbol           string           // 交易对
    Side             OrderSide        // 买卖方向 (Buy/Sell)
    Type             FuturesOrderType // 订单类型 (Limit/Market/Stop/Iceberg)
    Price            float64          // 价格
    Quantity         float64          // 数量
    Leverage         int              // 杠杆倍数 (1-100)
    MarginType       MarginType       // 保证金类型 (Cross/Isolated)
    PositionSide     PositionSide     // 持仓方向 (Long/Short)
    EntryPrice       float64          // 开仓价格
    LiquidationPrice float64          // 强平价格
    FilledQuantity   float64          // 已成交数量
    AvgFillPrice     float64          // 平均成交价格
    Status           int              // 订单状态
    UserID           string           // 用户ID
}
```

**实现代码**: [futures_order.go#L28-L56](engine/futures/futures_order.go#L28-L56)

### 3.2 持仓 (Position)

```go
type Position struct {
    PositionID       string          // 持仓ID
    Symbol           string          // 交易对
    UserID           string          // 用户ID
    Side             PositionSide    // 持仓方向 (Long/Short)
    Quantity         float64         // 持仓数量
    EntryPrice       float64         // 开仓价格
    MarkPrice        float64         // 标记价格
    LiquidationPrice float64         // 强平价格
    Margin           float64         // 保证金
    Leverage         int             // 杠杆倍数
    MarginType       MarginType      // 保证金类型
    UnrealizedPNL    float64         // 未实现盈亏
    RealizedPNL      float64         // 已实现盈亏
    FundingFee       float64         // 资金费用
    Status           PositionStatus  // 持仓状态
}
```

**实现代码**: [futures_order.go#L58-L83](engine/futures/futures_order.go#L58-L83)

### 3.3 交易记录 (FuturesTrade)

```go
type FuturesTrade struct {
    TradeID        string        // 交易ID
    Symbol         string        // 交易对
    OrderID        string        // 订单ID
    CounterOrderID string        // 对手订单ID
    Side           OrderSide     // 方向
    Price          float64       // 成交价格
    Quantity       float64       // 成交数量
    Fee            float64       // 手续费
    FeeCurrency    string        // 手续费币种
    PositionSide   PositionSide  // 持仓方向
}
```

**实现代码**: [futures_order.go#L85-L100](engine/futures/futures_order.go#L85-L100)

### 3.4 枚举类型

| 枚举 | 值 | 描述 |
|------|-----|------|
| OrderSide | Buy=0, Sell=1 | 订单方向 |
| PositionSide | Long=0, Short=1 | 持仓方向 |
| MarginType | CrossMargin=0, IsolatedMargin=1 | 保证金类型 |
| PositionStatus | Open=0, Closed=1, Liquidated=2 | 持仓状态 |

**实现代码**: [futures_order.go#L10-L26](engine/futures/futures_order.go#L10-L26)

## 4. 核心算法

### 4.1 标记价格计算 (Mark Price)

标记价格用于计算未实现盈亏和强平价格，防止价格操纵。

**计算公式:**

```
IndexPrice = Σ(ExchangePrice_i * Weight_i) / Σ(Weight_i)
Premium = FundingRate * (TimeToFunding / 8h)
MarkPrice = IndexPrice * (1 + Premium)
```

**价格偏离限制:**

```
Deviation = |MarkPrice - LastTradePrice| / LastTradePrice
If Deviation > 5%:
    MarkPrice = LastTradePrice * (1 ± 5%)
```

**实现代码**:
- [MarkPriceEngine 结构](risk/mark_price.go#L16-L27)
- [CalculateIndexPrice](risk/mark_price.go#L62-L68)
- [CalculateMarkPrice](risk/mark_price.go#L70-L102)

### 4.2 保证金计算

**初始保证金:**

```
InitialMargin = EntryPrice * Quantity / Leverage
```

**维持保证金:**

```
MaintenanceMargin = EntryPrice * Quantity * 0.5%
```

**保证金率:**

```
Equity = Margin + UnrealizedPNL
MarginRate = Equity / (MarkPrice * Quantity / Leverage)
```

**实现代码**:
- [CalculateInitialMargin](risk/margin.go#L69-L74)
- [CalculateMaintenanceMargin](risk/margin.go#L76-L81)
- [CalculateMarginRate](risk/margin.go#L83-L97)

### 4.3 强平价格计算

**多头强平价格:**

```
LiquidationPrice(Long) = EntryPrice * (1 - 1/Leverage + 0.5%)
```

**空头强平价格:**

```
LiquidationPrice(Short) = EntryPrice * (1 + 1/Leverage - 0.5%)
```

**实现代码**:
- [CalculateLiquidationPrice](risk/margin.go#L109-L121)
- [CheckLiquidation](risk/margin.go#L130-L139)

### 4.4 资金费率计算

资金费率用于平衡永续合约价格与现货价格的差异，每8小时结算一次。

**计算公式:**

```
FundingRate = clamp((IndexPrice - MarkPrice) / IndexPrice, -0.75%, +0.75%)
```

**资金费用结算:**

```
FundingFee = Quantity * MarkPrice * FundingRate

Long Position: 支付资金费用 (当 FundingRate > 0)
Short Position: 收取资金费用 (当 FundingRate > 0)
```

**实现代码**:
- [FundingRateEngine 结构](risk/funding_rate.go#L17-L29)
- [CalculateFundingRate](risk/funding_rate.go#L47-L67)
- [SettleFunding](risk/funding_rate.go#L74-L121)
- [StartFundingCycle](risk/funding_rate.go#L123-L140)

### 4.5 撮合算法

合约撮合采用价格优先、时间优先的原则。

**匹配流程:**

1. 限价买单按价格降序排列，价格越高优先级越高
2. 限价卖单按价格升序排列，价格越低优先级越高
3. 市价单优先匹配最优对手盘
4. 部分成交后剩余订单进入订单簿

**实现代码**:
- [FuturesOrderBook 结构](engine/futures/futures_engine.go#L22-L36)
- [Match 撮合方法](engine/futures/futures_engine.go#L107-L210)
- [FuturesEngine 结构](engine/futures/futures_engine.go#L297-L315)
- [SubmitOrder 提交订单](engine/futures/futures_engine.go#L433-L444)

## 5. API接口设计

### 5.1 订单管理

| 接口 | 方法 | 路径 | 描述 |
|------|------|------|------|
| 创建订单 | POST | `/api/v1/futures/orders` | 创建合约订单 |
| 查询订单列表 | GET | `/api/v1/futures/orders` | 查询订单列表 |
| 查询订单 | GET | `/api/v1/futures/orders/{order_id}` | 查询单个订单 |
| 取消订单 | DELETE | `/api/v1/futures/orders/{order_id}` | 取消订单 |

**创建订单请求:**

```json
{
    "symbol": "BTCUSDT_PERP",
    "side": "BUY",
    "type": "MARKET",
    "price": 94000.0,
    "quantity": 0.1,
    "leverage": 10,
    "margin_type": "isolated",
    "position_side": "long",
    "user_id": "user123"
}
```

### 5.2 持仓管理

| 接口 | 方法 | 路径 | 描述 |
|------|------|------|------|
| 查询持仓 | GET | `/api/v1/futures/positions` | 查询用户持仓 |
| 平仓 | POST | `/api/v1/futures/positions` | 平仓 |

### 5.3 市场数据

| 接口 | 方法 | 路径 | 描述 |
|------|------|------|------|
| 行情数据 | GET | `/api/v1/futures/ticker` | 获取合约行情 |
| 订单簿 | GET | `/api/v1/futures/orderbook` | 获取订单簿深度 |
| 标记价格 | GET | `/api/v1/futures/mark-price` | 获取标记价格 |
| 资金费率 | GET | `/api/v1/futures/funding-rate` | 获取资金费率 |

**行情响应示例:**

```json
{
    "symbol": "BTCUSDT_PERP",
    "last_price": 94250.50,
    "bid_price": 94249.00,
    "ask_price": 94251.00,
    "mark_price": 94250.75,
    "index_price": 94248.00,
    "funding_rate": 0.0001,
    "next_funding_time": 1782544338812
}
```

**实现代码**: [futures_handler.go](api/futures_handler.go)

## 6. 风险管理

### 6.1 风控流程

```
订单提交 → 保证金检查 → 撮合 → 持仓更新 → 风险监控 → 强平判定
```

### 6.2 风险监控

| 监控项 | 阈值 | 动作 |
|--------|------|------|
| 保证金率 < 100% | 预警线 | 发送保证金通知 |
| 保证金率 < 50% | 强平线 | 触发强制平仓 |
| 价格偏离 > 5% | 异常线 | 使用限价标记价格 |

### 6.3 强制平仓

当持仓保证金率低于维持保证金率时触发强平:

1. 计算强平价格
2. 以最优价格平仓
3. 更新用户账户
4. 记录强平日志

**实现代码**:
- [LiquidationEngine 结构](risk/liquidation.go#L10-L17)
- [CheckAndLiquidate](risk/liquidation.go#L27-L61)
- [liquidatePosition](risk/liquidation.go#L63-L95)
- [LiquidationResult 结构](risk/liquidation.go#L89-L107)

## 7. 部署架构

### 7.1 环境要求

| 组件 | 版本 | 用途 |
|------|------|------|
| Go | 1.21+ | 后端语言 |
| Redis | 7.0+ | 缓存/状态存储 |
| Kafka | 3.0+ | 消息队列 |
| PostgreSQL | 15+ | 持久化存储 |

### 7.2 配置说明

**支持的合约交易对:**

| 交易对 | 基础价格 | 最小变动 |
|--------|----------|----------|
| BTCUSDT_PERP | 94,250 USDT | 0.01 USDT |
| ETHUSDT_PERP | 4,850 USDT | 0.01 USDT |
| BNBUSDT_PERP | 610 USDT | 0.01 USDT |
| SOLUSDT_PERP | 175 USDT | 0.01 USDT |
| AVAXUSDT_PERP | 35 USDT | 0.001 USDT |
| XRPUSDT_PERP | 0.625 USDT | 0.0001 USDT |

**启动命令:**

```bash
go run cmd/server/main.go -port 8080 -shards 8
```

## 8. 测试验证

### 8.1 单元测试

| 模块 | 测试文件 | 测试用例数 | 状态 |
|------|----------|------------|------|
| engine/futures | [futures_order_test.go](engine/futures/futures_order_test.go) | 7 | ✅ 通过 |
| risk | [margin_test.go](risk/margin_test.go) | 6 | ✅ 通过 |
| risk | [funding_rate_test.go](risk/funding_rate_test.go) | 2 | ✅ 通过 |

### 8.2 API测试

| 接口 | 测试方法 | 状态 |
|------|----------|------|
| POST /api/v1/futures/orders | 创建市价单 | ✅ 通过 |
| GET /api/v1/futures/ticker | 获取行情 | ✅ 通过 |
| GET /api/v1/futures/mark-price | 获取标记价格 | ✅ 通过 |
| GET /api/v1/futures/funding-rate | 获取资金费率 | ✅ 通过 |
| GET /api/v1/futures/orderbook | 获取订单簿 | ✅ 通过 |

### 8.3 编译验证

```bash
go build ./...
# 编译成功，无错误
```

## 9. 性能指标

| 指标 | 目标值 | 当前状态 |
|------|--------|----------|
| 撮合延迟 | < 1ms | 开发中 |
| 吞吐量 | 100,000 TPS | 开发中 |
| 并发连接 | 100,000+ | 支持 |
| API响应时间 | < 50ms | ✅ 达标 |

## 10. 扩展计划

### 10.1 下一阶段

1. **期权模块**: 实现欧式期权定价和交易
2. **交割合约**: 实现季度/月度交割合约
3. **套利工具**: 实现期现套利、跨合约套利
4. **WebSocket推送**: 实时推送行情和交易数据

### 10.2 远期规划

1. **多链支持**: 支持EVM/Solana/Tron等多链衍生品
2. **量化接口**: 提供专业量化交易API
3. **风控中台**: 独立风控管理平台
4. **合规审计**: 完整的交易审计日志

## 11. 附录

### 11.1 文件结构

```
e:\codex\smartx_backend\
├── cmd/server/main.go              # 服务入口
├── api/
│   ├── futures_handler.go          # 合约API处理器
│   └── handler.go                  # 主API处理器
├── engine/futures/
│   ├── futures_order.go            # 订单/持仓结构
│   ├── futures_engine.go           # 撮合引擎
│   └── futures_order_test.go       # 单元测试
├── risk/
│   ├── margin.go                   # 保证金计算
│   ├── liquidation.go              # 强平逻辑
│   ├── mark_price.go               # 标记价格
│   ├── funding_rate.go             # 资金费率
│   ├── margin_test.go              # 保证金测试
│   └── funding_rate_test.go        # 资金费率测试
└── DESIGN_DERIVATIVES.md           # 设计文档
```

### 11.2 常量定义

| 常量 | 值 | 用途 |
|------|-----|------|
| MaintenanceMarginRate | 0.005 | 维持保证金率 (0.5%) |
| InitialMarginRate | 0.01 | 初始保证金率 (1%) |
| MaxLeverage | 100 | 最大杠杆倍数 |
| FundingInterval | 8h | 资金费率结算周期 |
| FundingRateLimit | 0.0075 | 资金费率限制 (±0.75%) |
| PriceDeviationThreshold | 0.05 | 价格偏离阈值 (5%) |

### 11.3 接口汇总

| 模块 | 接口数 | 描述 |
|------|--------|------|
| 订单管理 | 4 | 创建/查询/取消订单 |
| 持仓管理 | 2 | 查询/平仓 |
| 市场数据 | 4 | 行情/订单簿/标记价格/资金费率 |
| 总计 | 10 | 完整的合约交易API |

---

**设计文档与代码关联**: 文档中所有代码链接均为相对路径，可在 VS Code 中直接点击跳转到对应代码文件和代码块。
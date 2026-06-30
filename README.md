# SmartX 后端 - 机构级衍生品交易系统

SmartX 后端是一套企业级的衍生品交易系统，基于 Go 语言构建，支持永续合约、交割合约、期权等多种衍生品交易功能。

## ✨ 核心特性

### 永续合约
- **杠杆交易**: 支持 1–100x 灵活杠杆
- **多空双向**: 支持做多和做空
- **资金费率**: 每 8 小时结算一次资金费用
- **标记价格**: 采用指数价格+溢价的方式计算标记价格，防止插针攻击
- **保证金机制**: 初始保证金和维持保证金双重保障

### 交割合约
- **季度合约**: 每季度交割的期货合约
- **月度合约**: 每月交割的期货合约
- **结算机制**: 到期按交割价格结算
- **基差追踪**: 实时追踪期现价差

### 期权
- **欧式期权**: 到期日才能行权的期权合约
- **定价模型**: 基于 Black-Scholes 模型定价
- **希腊字母**: 支持 Delta、Gamma、Vega、Theta、Rho 计算
- **期权链**: 完整的期权合约管理

### 套利工具
- **期现套利**: 现货和期货之间的套利策略
- **跨合约套利**: 不同合约之间的套利机会
- **交割基差工具**: 交割期基差分析和预测

### 风险管理
- **实时保证金监控**: 实时监控账户保证金状态
- **强平引擎**: 自动强平风险超限账户
- **仓位限制**: 单笔和累计仓位限制
- **风险敞口追踪**: 全面追踪账户风险敞口

### 高性能撮合引擎
- **跳表订单簿**: 高效的订单簿管理
- **价格时间优先**: 标准的撮合规则
- **分片处理**: 按交易对分片，支持并行撮合
- **低延迟**: 纳秒级订单处理延迟

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────────────────────────┐
│                         API 层                                      │
│  REST API | WebSocket | gRPC                                        │
│  - HTTP 接口 | - 实时推送 | - 高性能 RPC                             │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       撮合引擎                                      │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐                       │
│  │  Shard #1 │  │  Shard #2 │  │   ...    │                       │
│  │ OrderBook │  │ OrderBook │  │          │                       │
│  │ Matching  │  │ Matching  │  │          │                       │
│  └───────────┘  └───────────┘  └───────────┘                       │
│  SkipList | Order | MatchingEngine | Shard                          │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       业务逻辑层                                      │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐             │
│  │ FuturesEngine │ │ OptionsEngine │ │   RiskEngine  │             │
│  │(合约引擎)     │ │(期权引擎)     │ │ (风控引擎)    │             │
│  └───────────────┘ └───────────────┘ └───────────────┘             │
│  - 订单管理 | - 持仓管理 | - 结算管理 | - 强平管理                   │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       消息队列层                                      │
│                          Kafka                                      │
│  Topics: orders, trades, positions, settlements, liquidations      │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         存储层                                      │
│  ┌──────────┐ ┌──────────┐ ┌──────────────┐                        │
│  │  MySQL   │ │  Redis   │ │ ClickHouse   │                        │
│  │(业务数据) │ │(缓存/锁) │ │ (时序数据)   │                        │
│  └──────────┘ └──────────┘ └──────────────┘                        │
└─────────────────────────────────────────────────────────────────────┘
```

## 📁 项目结构

```
smartx_backend/
├── api/                    # API 处理层
│   ├── handler.go           # 通用处理器
│   └── futures_handler.go   # 合约业务处理器
├── cmd/                    # 命令行入口
│   └── server/              # 服务器主程序
│       └── main.go
├── config/                 # 配置管理
│   └── config.go            # 配置加载
├── engine/                 # 撮合引擎
│   ├── matching.go          # 撮合逻辑
│   ├── order.go             # 订单模型
│   ├── orderbook.go         # 订单簿
│   ├── shard.go             # 分片管理
│   └── skiplist.go          # 跳表实现
├── engine/futures/          # 合约引擎
│   ├── futures_engine.go    # 合约引擎主逻辑
│   ├── futures_order.go     # 合约订单
│   └── futures_order_test.go # 测试
├── mq/                     # 消息队列
│   └── kafka.go             # Kafka 客户端
├── proto/                  # 协议定义
│   ├── market.go            # 行情协议
│   └── serializer.go        # 序列化
├── risk/                   # 风险管理
│   ├── funding_rate.go      # 资金费率
│   ├── funding_rate_test.go # 资金费率测试
│   ├── liquidation.go       # 强平逻辑
│   ├── margin.go            # 保证金计算
│   ├── margin_test.go       # 保证金测试
│   └── mark_price.go        # 标记价格
├── ws/                     # WebSocket
│   └── hub.go               # WebSocket Hub
├── API.md                  # API 文档
├── DESIGN.md               # 设计文档
├── DESIGN_DERIVATIVES.md   # 衍生品设计文档
├── Dockerfile              # Docker 镜像构建
├── docker-compose.yml      # Docker Compose 配置
├── docker-entrypoint.sh    # Docker 入口脚本
├── config.toml             # 配置文件
├── go.mod                  # Go 依赖管理
├── go.sum                  # Go 依赖校验
└── server.exe              # Windows 可执行文件
```

## 🚀 快速开始

### 环境要求

- **Go**: 1.21+
- **Docker**: 24.0+
- **Docker Compose**: 2.20+
- **MySQL**: 8.0+
- **Redis**: 7.0+
- **Kafka**: 3.5+

### 使用 Docker 启动

```bash
# 进入项目目录
cd smartx_backend

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f server
```

### 本地开发

```bash
# 下载依赖
go mod download

# 运行服务器
go run cmd/server/main.go --config config.toml

# 构建可执行文件
go build -o server.exe cmd/server/main.go
```

### 配置说明

配置文件位于 `config.toml`，主要配置项包括：

```toml
[server]
port = 8080
read_timeout = 30
write_timeout = 30

[database]
host = "localhost"
port = 3306
user = "root"
password = "password"
database = "smartx"

[redis]
addr = "localhost:6379"
password = ""
db = 0

[kafka]
brokers = ["localhost:9092"]
topic_prefix = "smartx"

[matching]
shard_count = 16
orderbook_depth = 100
max_order_size = 1000000

[risk]
initial_margin_rate = 0.02
maintenance_margin_rate = 0.01
funding_interval = 28800
max_leverage = 100
```

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行特定模块测试
go test ./engine/...
go test ./risk/...

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 运行性能测试
go test -bench=. ./engine/...
```

## 📊 核心模块

### 撮合引擎

撮合引擎是系统的核心组件，负责订单的匹配和成交：

- **跳表实现**: 使用跳表数据结构存储订单簿，支持 O(log n) 的插入、删除和查找
- **价格时间优先**: 优先匹配价格最优的订单，同价格按时间顺序匹配
- **分片处理**: 按交易对分片，每个分片独立运行，提高系统吞吐量

### 合约引擎

合约引擎负责合约交易的核心业务逻辑：

- **订单管理**: 限价单、市价单、止盈止损单等多种订单类型
- **持仓管理**: 多空持仓的实时计算和更新
- **结算管理**: 资金费率结算和交割结算
- **强平管理**: 保证金不足时的强制平仓

### 风险管理

风险管理模块保障系统的安全运行：

- **保证金计算**: 实时计算初始保证金和维持保证金
- **标记价格**: 基于指数价格和溢价计算，防止市场操纵
- **资金费率**: 根据市场情况动态调整资金费率
- **强平引擎**: 自动触发强平流程

## 🔧 API 接口

### 订单接口

```bash
# 创建订单
POST /api/v1/futures/order
Content-Type: application/json

{
  "symbol": "BTC-USDT",
  "side": "BUY",
  "type": "LIMIT",
  "price": "45000",
  "quantity": "1",
  "leverage": 10
}

# 取消订单
DELETE /api/v1/futures/order/{orderId}

# 获取订单状态
GET /api/v1/futures/order/{orderId}

# 获取当前订单
GET /api/v1/futures/openOrders
```

### 持仓接口

```bash
# 获取持仓
GET /api/v1/futures/positions

# 获取单个持仓
GET /api/v1/futures/position/{symbol}

# 调整杠杆
POST /api/v1/futures/leverage
Content-Type: application/json

{
  "symbol": "BTC-USDT",
  "leverage": 20
}
```

### 账户接口

```bash
# 获取账户信息
GET /api/v1/futures/account

# 获取资金费率
GET /api/v1/futures/fundingRate/{symbol}

# 获取标记价格
GET /api/v1/futures/markPrice/{symbol}
```

完整 API 文档请参考 [API.md](API.md)

## 📖 文档

- [设计文档](DESIGN.md) - 系统架构和设计细节
- [衍生品设计文档](DESIGN_DERIVATIVES.md) - 衍生品交易功能设计
- [API 文档](API.md) - REST API 接口说明

## 📝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE.txt](LICENSE.txt)

## 📞 联系方式

如有问题或建议，请通过以下方式联系：

- 提交 [Issue](https://github.com/your-username/smartx_backend/issues)
- 发送邮件至 support@smartx.io
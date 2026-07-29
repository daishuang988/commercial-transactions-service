# 商业交易服务平台 - 环境依赖与启动指南

## 环境信息

| 项目 | 版本/值 | 说明 |
|------|---------|------|
| 操作系统 | Windows 10 Pro | |
| Go | 1.25.0 (windows/amd64) | `C:\Go\bin\go.exe` |
| MySQL | 8.0.46 (Windows service) | `C:\mysql\mysql-8.0.46-winx64\` |
| Redis | 5.0.14.1 (手动启动) | `C:\Redis\redis-server.exe` |

---

## 一、MySQL

### 基本信息

| 项目 | 值 |
|------|-----|
| 安装路径 | `C:\mysql\mysql-8.0.46-winx64\` |
| 配置文件 | `C:\mysql\mysql-8.0.46-winx64\my.ini` |
| 数据目录 | `C:\mysql\data\` |
| 端口 | `3306` |
| 连接参数 | `root / root` |
| 数据库名 | `flash_sale` |
| 字符集 | `utf8mb4` |
| 服务名 | `MySQL` |

### 启动/停止命令

```bash
# 启动（以 Windows 服务方式）
net start MySQL

# 停止
net stop MySQL

# 重启
net stop MySQL && net start MySQL
```

### 连接测试

```bash
mysql -h127.0.0.1 -uroot -proot flash_sale -e "SELECT 1"
```

---

## 二、Redis

### 基本信息

| 项目 | 值 |
|------|-----|
| 安装路径 | `C:\Redis\` |
| 配置文件 | `C:\Redis\redis.windows.conf` |
| 端口 | `6379` |
| 密码 | 无 |
| DB | `0` |
| 数据文件 | `dump.rdb`（工作目录生成） |

### 启动命令

```bash
/c/Redis/redis-server.exe /c/Redis/redis.windows.conf
```

> **注意**：Redis 未注册为 Windows 服务，需手动启动。建议在项目根目录 `E:\Commercial_Transactions_Service\` 下运行，RDB 文件会写入该目录。

### 停止命令

```bash
# 通过 redis-cli 正常关闭
/c/Redis/redis-cli.exe SHUTDOWN

# 或强制杀进程
taskkill /F /IM redis-server.exe
```

### 连接测试

```bash
/c/Redis/redis-cli.exe PING
# 返回 PONG 表示正常
```

---

## 三、Go 项目依赖

### 编译工具

| 项目 | 路径 |
|------|------|
| Go | `C:\Go\bin\go.exe` |

### 核心依赖 (go.mod)

| 包 | 版本 | 用途 |
|----|------|------|
| `github.com/gin-gonic/gin` | v1.12.0 | HTTP 框架 |
| `gorm.io/gorm` | v1.31.2 | ORM |
| `gorm.io/driver/mysql` | v1.6.0 | GORM MySQL 驱动 |
| `github.com/redis/go-redis/v9` | v9.21.0 | Redis 客户端（Lua 脚本 + Stream） |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT 认证 (HS256) |
| `gopkg.in/yaml.v3` | v3.0.1 | 配置文件解析 |
| `golang.org/x/crypto` | v0.53.0 | bcrypt 密码加密 |
| `github.com/xuri/excelize/v2` | v2.11.0 | Excel 导出 |
| `github.com/pdfcpu/pdfcpu` | v0.13.0 | PDF 合同处理 |

### 安装依赖

```bash
/c/Go/bin/go.exe mod download
```

---

## 四、项目配置

配置文件：[config.yaml](../config.yaml)

```yaml
server:
  port: 8080
  mode: debug          # 生产环境改为 release

mysql:
  host: 127.0.0.1
  port: 3306
  user: root
  password: root
  database: flash_sale
  max_open_conns: 200
  max_idle_conns: 50

redis:
  host: 127.0.0.1
  port: 6379
  password: ""
  db: 0
  pool_size: 500       # 秒杀高并发

jwt:
  secret: "flash-sale-secret-key-change-in-production"
  expire_hours: 2

flash_sale:
  max_per_user: 1
  worker_count: 10        # 订单落库 Worker 数量
  batch_size: 200         # 批量写入条数
  batch_interval_ms: 500  # 写入间隔(毫秒)
```

---

## 五、项目启动

### 完整启动流程

按顺序执行以下三步：

```bash
# 1. 启动 MySQL（如果未运行）
net start MySQL

# 2. 启动 Redis（如果未运行）
/c/Redis/redis-server.exe /c/Redis/redis.windows.conf &

# 3. 编译并启动项目
cd /e/Commercial_Transactions_Service
/c/Go/bin/go.exe build -o server.exe ./cmd/server/ && ./server.exe
```

### 快捷一键启动（项目目录下执行）

```bash
net start MySQL 2>/dev/null; \
/c/Redis/redis-server.exe /c/Redis/redis.windows.conf & \
sleep 1; \
/c/Go/bin/go.exe build -o server.exe ./cmd/server/ && ./server.exe
```

### 仅重新编译并启动（MySQL/Redis 已运行）

```bash
cd /e/Commercial_Transactions_Service
taskkill /F /IM server.exe 2>/dev/null; sleep 1
/c/Go/bin/go.exe build -o server.exe ./cmd/server/ && ./server.exe
```

### 启动成功标志

看到以下日志即为启动成功：

```
MySQL 连接成功
Redis 连接成功
启动 10 个 Redis 订单落库Worker
🚀 服务启动: :8080
```

### 健康检查

```bash
curl http://localhost:8080/health
```

---

## 六、全部停止

```bash
# 停止 server
taskkill /F /IM server.exe

# 停止 Redis
/c/Redis/redis-cli.exe SHUTDOWN

# 停止 MySQL
net stop MySQL
```

---

## 七、启动顺序总结

```
┌─────────┐    ┌─────────┐    ┌───────────┐
│  MySQL  │ →  │  Redis  │ →  │ Go Server │
│  :3306  │    │  :6379  │    │   :8080   │
│ 服务自启│    │  手动启  │    │   手动启   │
└─────────┘    └─────────┘    └───────────┘
```

> MySQL 注册了 Windows 服务开机自启；Redis 和 Go Server 需手动启动。

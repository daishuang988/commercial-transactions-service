# 项目全栈手册

## 一、环境启动

### MySQL
```bash
# 启动（Windows 服务）
net start MySQL

# 停止
net stop MySQL

# 连接
mysql -h127.0.0.1 -uroot -proot flash_sale
```

- 安装路径：`C:\mysql\mysql-8.0.46-winx64\`
- 配置文件：`C:\mysql\mysql-8.0.46-winx64\my.ini`
- 端口：`3306`

### Redis
```bash
# 启动
/c/Redis/redis-server.exe /c/Redis/redis.windows.conf &

# 测试
/c/Redis/redis-cli.exe PING

# 停止
/c/Redis/redis-cli.exe SHUTDOWN
```

- 安装路径：`C:\Redis\`
- 端口：`6379`

### Go 项目
```bash
# 编译
cd /e/Commercial_Transactions_Service
/c/Go/bin/go.exe build -o server.exe ./cmd/server/

# 启动
./server.exe

# 健康检查
curl http://localhost:8080/health
```

- 入口：`cmd/server/main.go`
- 配置：`config/config.yaml`
- 端口：`8080`

---

## 二、项目结构

```
cmd/server/main.go          # 入口 + 路由注册
config/config.yaml           # 配置
internal/
  config/                    # 配置加载
  handler/
    front/                   # C端接口
      auth.go                # 登录/注册/重置密码
      flashsale.go           # 秒杀+剩余次数
      order.go               # 订单+寄卖+分佣
      product.go             # 商品/寄售/配置
      wallet_logs.go         # 优惠券/奖金明细
      withdraw.go            # 提现
      settings.go            # 个人信息/地址/收支方式
      payment.go             # 收款方式管理
    admin/                   # 管理端接口
      admin.go               # 用户/订单/提现/充值
      admin_extra.go         # 配置/导出/合同
  middleware/                 # JWT/CORS/日志/限流
  service/                   # 秒杀核心+异步Worker+凌晨优惠券
  repository/                # DB/Redis/Memory数据访问
  model/                     # GORM数据模型
pkg/
  app/                       # 统一响应+错误码
  utils/                     # MD5/Salt/RandStr
docs/                        # 文档
scripts/                     # 迁移脚本
tools/                       # 工具
  old_system_migration/      # 老系统爬虫
  gen_sql.go                 # JSON→SQL转换
```

---

## 三、接口清单

### C端（Base: `/api/v1/front`）

**公开（无需认证）**
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /auth/login | 登录 |
| POST | /auth/register-v2 | 注册 |
| POST | /auth/reset-password | 重置密码 |
| POST | /sms/send | 发短信 |
| GET | /flash-sale/time | 秒杀时间 |
| GET | /config/service-phone | 客服电话 |
| GET | /config/contract-content | 合同内容 |
| GET | /config/trade-rules | 交易规则配置 |
| GET | /banners | 轮播图 |
| GET | /agreements | 协议 |
| GET | /categories | 分类 |
| GET | /announcement | 公告 |

**需认证**
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /flash-sale/products | 秒杀商品列表 |
| GET | /flash-sale/remaining | 剩余可抢次数 |
| POST | /flash-sale/buy | 秒杀抢购 |
| GET | /products | 商品列表 |
| GET | /products/:id | 商品详情 |
| GET | /merchandises | 寄售商品列表 |
| GET | /merchandises/:id | 寄售商品详情 |
| POST | /merchandises/:id/buy | 购买寄售商品 |
| GET | /orders | 我的订单 |
| GET | /orders/:id | 订单详情 |
| POST | /orders/:id/pay | 上传付款凭证 |
| POST | /orders/:id/confirm | 确认收款 |
| POST | /orders/:id/resell | 寄卖 |
| POST | /orders/:id/cancel | 取消订单 |
| GET | /user/profile | 个人信息+钱包 |
| PUT | /user/profile | 修改个人信息 |
| GET | /user/wallet | 钱包 |
| GET | /user/fans | 我的粉丝 |
| GET | /user/addresses | 收货地址列表 |
| POST | /user/address | 新增地址 |
| PUT | /user/address/:id | 修改地址 |
| DELETE | /user/address/:id | 删除地址 |
| GET | /user/payment-methods | 收款方式列表 |
| POST | /user/payment-method | 新增收款方式 |
| DELETE | /user/payment-method/:id | 删除收款方式 |
| GET | /user/contract | 合同 |
| GET | /user/contract-status | 合同状态 |
| POST | /user/contract/sign | 签署合同 |
| PUT | /user/password | 修改密码 |
| POST | /upload | 上传文件 |
| POST | /withdraw | 提现申请 |
| GET | /logs/coupon | 优惠券明细 |
| GET | /logs/self-bonus | 个人奖金明细 |
| GET | /logs/share-bonus | 推广奖金明细 |

### 管理端（Base: `/api/v1/admin`）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /auth/login | 登录 |
| GET | /dashboard | 数据看板 |
| GET | /config | 系统配置 |
| PUT | /config | 更新配置 |
| GET | /account/info | 管理员信息 |
| GET | /users | 用户列表 |
| GET | /users/export | 用户导出 |
| GET | /users/:id | 用户详情 |
| PUT | /users/:id | 编辑用户 |
| PUT | /users/:id/status | 冻结/解冻 |
| PUT | /users/:id/parent | 修改上级 |
| POST | /users/:id/recharge | 充值 |
| POST | /users/batch-delete | 批量删除 |
| GET | /orders | 订单列表 |
| GET | /orders/:id | 订单详情 |
| PUT | /orders/:id/status | 修改订单状态 |
| GET | /withdraws | 提现列表 |
| GET | /withdraws/export | 提现导出 |
| PUT | /withdraws/:id/approve | 提现审批 |
| GET | /logs/coupon | 优惠券明细 |
| GET | /logs/coupon/export | 优惠券导出 |
| GET | /logs/self-bonus | 个人奖金明细 |
| GET | /logs/self-bonus/export | 个人奖金导出 |
| GET | /logs/share-bonus | 推广奖金明细 |
| GET | /logs/share-bonus/export | 推广奖金导出 |
| POST | /orders/settle-coupons | 手动触发优惠券结算 |

---

## 四、MySQL 表结构

| 表名 | 说明 |
|------|------|
| users | 用户(30列) |
| user_wallets | 钱包(money/coupon/self_bonus/share_bonus) |
| user_addresses | 收货地址 |
| user_contracts | 用户合同 |
| withdraw_accounts | 收款方式 |
| orders | 订单(含coupon_settled标记) |
| merchandises | 寄售商品(status:0可售/1已售) |
| exchange_orders | 兑换订单(寄卖流转) |
| withdraws | 提现记录 |
| coupon_logs | 优惠券明细 |
| self_bonus_logs | 个人奖金明细 |
| share_bonus_logs | 推广奖金明细 |
| money_logs | 余额明细 |
| flash_sale_events | 秒杀活动 |
| flash_sale_records | 秒杀记录 |
| goods | 商品模板 |
| categories | 商品分类 |
| banners | 轮播图 |
| ads | 广告图 |
| system_configs | 系统配置(key-value) |
| admin_users | 管理员 |
| roles | 角色 |
| rules | 菜单规则 |

---

## 五、Redis

| Key | 说明 |
|-----|------|
| `product:stock:{id}` | 秒杀库存 |
| `order:stream` | 订单落库Stream |
| `flash:user:daily:{uid}:{date}` | 用户每日抢购计数(TTL到当天23:59:59) |

---

## 六、工具脚本

| 脚本 | 用途 | 用法 |
|------|------|------|
| `tools/gen_sql.go` | JSON→SQL 转换 | `go run tools/gen_sql.go \| mysql ...` |
| `tools/import_accounts.go` | 导入收款账户 | `go run tools/import_accounts.go` |
| `tools/old_system_migration/cmd/crawl/` | 爬取老系统 | `go build -o crawl.exe && ./crawl.exe -fullsync -api-list apis.txt -cookie "PHPSID=xxx"` |

---

## 七、定时任务

### 优惠券凌晨结算

每天 **23:59** 自动执行，结算当天所有已完成订单的优惠券（抢单奖励 = 订单金额 × `order_reward_rate`）。

**代码位置**：`internal/service/flashsale.go` → `StartDailyCouponSettlement()`

**手动触发**（管理端接口，需登录）：
```bash
curl -X POST http://localhost:8080/api/v1/admin/orders/settle-coupons \
  -H "Authorization: Bearer {admin_token}"
```

**执行逻辑**：遍历 `orders` 表中 `status=2 AND coupon_settled=0` 的订单，计算优惠券写入 `coupon_logs` 和 `user_wallets`，标记 `coupon_settled=1`。

# 商业交易服务平台 - 项目参考文档

## 技术栈

- 语言: Go 1.25
- HTTP框架: Gin v1.12
- ORM: GORM v1.31 (MySQL)
- 缓存/队列: Redis (go-redis v9)
- 认证: JWT (golang-jwt v5, HS256)
- 密码: bcrypt + 兼容老系统 MD5(密码+盐)

## 项目结构

```
cmd/server/main.go        # 入口 + 路由注册
internal/
  handler/front/           # C端接口
  handler/admin/           # 管理端接口
  middleware/               # JWT/CORS/日志/限流
  service/                  # 秒杀核心+异步Worker
  repository/               # DB/Redis/Memory数据访问
  model/                    # GORM数据模型
  config/                   # 配置加载
pkg/app/                    # 统一响应+错误码
pkg/utils/                  # bcrypt/MD5/RandStr
tools/
  old_system_migration/     # 老系统爬虫+迁移工具
  loadtest/                 # 压测脚本
config/config.yaml          # 配置文件
```

## 数据库

- MySQL: flash_sale 库
- Redis: 秒杀库存/记录
- 核心表: users(1868), user_wallets, orders(89289), merchandises(75700), coupon_logs, self_bonus_logs, share_bonus_logs, withdraws, system_configs, admin_users, roles, rules

## C端接口

### 公开(无需认证)
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/front/auth/login | 登录 |
| POST | /api/v1/front/auth/register-v2 | 注册(短信mock 1234) |
| POST | /api/v1/front/auth/reset-password | 重置密码(mock 1234) |
| GET | /api/v1/front/banners | 轮播图 |
| GET | /api/v1/front/categories | 分类(只返回有上架商品的) |
| GET | /api/v1/front/announcement | 公告(config_key: notice_content) |
| GET | /api/v1/front/agreements | 协议(agreement_user+agreement_consignment) |
| GET | /api/v1/front/flash-sale/time | 秒杀时间(含优先用户时间) |

### 需认证
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/front/user/profile | 个人信息+钱包 |
| GET | /api/v1/front/user/wallet | 钱包(从日志表实时计算) |
| PUT | /api/v1/front/user/password | 修改密码 |
| GET | /api/v1/front/products | 商品列表(只显示status=1) |
| GET | /api/v1/front/products/:id | 商品详情 |
| GET | /api/v1/front/merchandises | 寄售商品(status=0 AND is_show=1) |
| GET | /api/v1/front/orders | 订单列表(role=buyer/seller, status=0/1/2) |
| GET | /api/v1/front/orders/:id | 订单详情(含买卖双方昵称/手机/商品) |
| POST | /api/v1/front/orders/:id/pay | 买方上传付款凭证(0→1) |
| POST | /api/v1/front/orders/:id/confirm | 卖方确认收款(1→2) |
| GET | /api/v1/front/logs/coupon | 优惠券明细(limit=10, type=1/2) |
| GET | /api/v1/front/logs/self-bonus | 个人奖金明细 |
| GET | /api/v1/front/logs/share-bonus | 推广奖金明细 |
| GET | /api/v1/front/flash-sale/products | 秒杀商品 |
| POST | /api/v1/front/flash-sale/buy | 抢购(三层校验:全局+活动+优先) |

### 订单状态流转
```
status=0 待付款: 买方仓库/卖方仓库
status=1 待确认: 买方交易中/卖方交易中
status=2 已完成: 买方已完成/卖方已完成

买方上传凭证 POST /orders/:id/pay  → 0→1
卖方确认收款 POST /orders/:id/confirm → 1→2
```

### 前端Tab对应
```
买方: /orders?role=buyer&status=0  (买方仓库)
      /orders?role=buyer&status=1  (交易中)
      /orders?role=buyer&status=2  (已完成)
卖方: /orders?role=seller&status=0 (卖方仓库)
      /orders?role=seller&status=1 (交易中)
      /orders?role=seller&status=2 (已完成)
```

## 管理端接口(部分关键)
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/admin/auth/login | 管理端登录 |
| GET | /api/v1/admin/config | 系统配置(含_meta) |
| PUT | /api/v1/admin/config | 更新配置(覆盖) |
| GET | /api/v1/admin/users | 用户管理 |
| PUT | /api/v1/admin/users/:id | 编辑用户 |
| PUT | /api/v1/admin/orders/:id/status | 修改订单状态 |
| POST | /api/v1/admin/flash-sale/events | 创建秒杀活动 |
| GET | /api/v1/admin/admins | 管理员列表 |
| POST | /api/v1/admin/admins | 创建管理员 |
| DELETE | /api/v1/admin/admins/:id | 删除管理员 |
| GET | /api/v1/admin/roles | 角色列表 |
| DELETE | /api/v1/admin/roles/:id | 删除角色(校验关联) |
| GET | /api/v1/admin/rules/tree | 菜单树(按角色权限过滤) |

## 秒杀规则
- 日期格式: 1-7 = 周一至周日, 7自动映射为0
- 时间格式: HH:MM 24小时制
- priority_advance_minutes: 优先用户提前N分钟
- priority_max_orders: 优先用户最多N单(与用户个人max_order取MIN)
- 所有时间统一使用北京时间(Asia/Shanghai或UTC+8)

## 生产环境配置
- MySQL连接池: max_open=200, max_idle=50
- Redis连接池: pool_size=500
- 限流: POST /buy → 500 QPS
- 缓存: flash-sale/time + flash-sale/products → 1秒内存缓存
- 健康检查: GET /health
- 请求日志: 每条记录时间/路径/状态/耗时/IP

## 数据迁移流程
1. 爬虫: crawl -fullsync -api-list apis.txt -cookie COOKIE
2. 迁移: go run scripts/remigrate_all.go (MYSQL_DSN=xxx)
3. 密码: UPDATE users SET password = MD5(CONCAT('123456', salt))
4. 支出memo: UPDATE xxx_logs SET memo='用户提现' WHERE type=2 AND memo=''

## 测试账号
- 超管: 18920137809 / 123456
- C端买方/卖方: 13701142651 / 123456 (ID=93512, 买1686条/卖1211条)
- 优先用户: 18920121111 / 123456 (ID=99960, is_priority=1, max_order=2)
- 邀请码示例: vv8cxp (用户18194867112)

## 已知配置项(交易规则未接入业务)
resell_rate(0.02), store_manager_rate(0.01), direct_referral_rate(0.002),
static_income_rate(0.01), order_reward_rate(0.005), listing_fee_rate(0.01),
resell_deadline(14:45), resell_product_id(1), flash_sale_product(1,3044,套),
split_threshold(39000,2)

## 部署
- 推荐: 阿里云 ECS 4核8G + RDS MySQL 4核8G + Redis 4G主从
- systemd 自动重启: Restart=always
- 配置: config.yaml (环境变量MYSQL_DSN用于迁移脚本)

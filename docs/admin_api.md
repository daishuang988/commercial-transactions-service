# 管理端 API 接口文档

> **Base URL**: `http://localhost:8080/api/v1/admin`  
> **认证方式**: `Authorization: Bearer {token}`（登录后获取）  
> **响应格式**: `{"code":0,"msg":"ok","data":...,"count":...}`  
> **code=0 成功，非0 失败**

---

## 1. 登录

### POST /auth/login

**使用页面**: 管理端登录页

**入参**:
```json
{
  "username": "admin",
  "password": "admin123"
}
```

**出参**:
```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOi...",
    "user_id": 1,
    "username": "admin",
    "nickname": "超级管理员"
  }
}
```

---

## 2. 数据看板（Dashboard 首页）

### GET /dashboard

**使用页面**: 管理端首页仪表盘，展示今日核心指标

**无需入参**

**出参**:
```json
{
  "code": 0,
  "data": {
    "today_users": 7,          // 今日新增用户
    "today_orders": 1262,      // 今日订单数
    "today_sales": 35623810.35,// 今日交易额(元)
    "pending_withdraw": 394,   // 待审批提现数
    "total_users": 1844,       // 总用户数
    "total_orders": 86650      // 总订单数
  }
}
```

---

## 3. 系统配置

### GET /config

**使用页面**: 商城设置 → 系统设置页（Logo、主题色、菜单风格、多标签页配置等）

**无需入参**

**出参**:
```json
{
  "code": 0,
  "data": {
    "logo": { "title": "晟瑞达商贸", "image": "/app/admin/..." },
    "menu": { "data": "/app/admin/rule/get", "method": "GET", "accordion": true, "collapse": false, "control": false, "controlWidth": 500, "select": "0", "async": true },
    "tab": { "enable": true, "keepState": true, "preload": false, "session": false, "max": "30" },
    "theme": { "defaultColor": "2", "defaultMenu": "light-theme", "defaultHeader": "light-theme", "allowCustom": true, "banner": false },
    "colors": [{ "id": "1", "color": "#36b368", "second": "#f0f9eb" }, ...],
    "other": { "keepLoad": "500", "autoHead": false, "footer": false },
    "header": { "message": false }
  }
}
```

### GET /account/info

**使用页面**: 右上角管理员头像下拉 → 个人资料

**无需入参**

**出参**:
```json
{
  "code": 0,
  "data": {
    "id": 1,
    "username": "admin",
    "nickname": "超级管理员",
    "avatar": "/app/admin/avatar.png",
    "email": null,
    "mobile": null,
    "roles": "1",
    "login_at": "2026-07-19T11:32:34+08:00"
  }
}
```

---

## 4. 用户管理

> 对应菜单：**用户管理 → 用户管理**

### GET /users

**使用页面**: 用户列表页（表格+分页+搜索+筛选）

**入参** (Query String):

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 是 | 页码，从1开始 |
| limit | int | 是 | 每页条数，最大100 |
| keyword | string | 否 | 搜索：用户名/昵称/手机号模糊匹配 |
| level | int | 否 | 筛选用户等级 |
| status | int | 否 | 筛选状态：0冻结 1正常 |
| pid | int | 否 | 筛选某上级的所有下级 |

**示例**: `GET /users?page=1&limit=20&keyword=138&status=1`

**出参**:
```json
{
  "code": 0,
  "msg": "ok",
  "count": 1844,
  "data": [{
    "id": 99932,
    "username": "13676078283",
    "nickname": "136****8283",
    "mobile": "13676078283",
    "sex": 1,
    "avatar": "/app/admin/avatar.png",
    "invite": "fwpk5m",           // 推广码
    "level": 1,                   // 用户等级
    "birthday": null,
    "is_vip": 0,                  // 0否 1是
    "viptime": null,              // VIP到期时间
    "is_resell": 1,               // 是否转售 0否 1是
    "max_order": 0,               // 每日最高可抢单数
    "contract": "/upload/sign_contract/xxx.pdf",
    "pid": 98949,                 // 上级用户ID
    "join_time": "2026-07-20T12:04:04+08:00",
    "join_ip": "122.100.132.48",
    "last_time": "2026-07-20T14:00:58+08:00",
    "last_ip": "122.100.132.48",
    "status": 1,                  // 0冻结 1正常
    "created_at": "2026-07-20T12:04:04+08:00",
    "updated_at": "2026-07-20T14:00:58+08:00"
  }]
}
```

### GET /users/:id

**使用页面**: 用户详情页（点击用户行展开或跳转）

**入参**: URL路径中的 `id`（用户ID）

**出参**: 同上单条 + 钱包信息
```json
{
  "code": 0,
  "data": {
    "user": { /* 同上用户对象 */ },
    "wallet": {
      "user_id": 99932,
      "money": 0.000,             // 余额
      "coupon": 0.000,            // 优惠券
      "self_bonus": 0.000,        // 个人奖金
      "share_bonus": 0.000,       // 推广奖金
      "score": 0,                 // 积分
      "poor": 0.00                // 待还款(负数=欠款)
    }
  }
}
```

### PUT /users/:id/status

**使用页面**: 用户列表 → 操作列 → 冻结/解冻按钮

**入参**:
```json
{
  "status": 0    // 0=冻结 1=解冻
}
```

---

## 5. 订单管理

> 对应菜单：**订单管理 → 订单管理**

### GET /orders

**使用页面**: 订单列表页（表格+分页+筛选）

**入参** (Query String):

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 是 | 页码 |
| limit | int | 是 | 每页条数 |
| status | int | 否 | 1待确认 2已完成 |
| seller_id | int | 否 | 卖家ID |
| buyer_id | int | 否 | 买家ID |
| keyword | string | 否 | 搜索：订单号/收货人/电话 |
| start_time | string | 否 | 开始时间 `2026-07-01` |
| end_time | string | 否 | 结束时间 `2026-07-20` |

**出参**:
```json
{
  "code": 0,
  "count": 86650,
  "data": [{
    "id": 397481,
    "order_sn": "36779117845130258730",
    "seller_id": 96764,
    "buyer_id": 97763,
    "merchandise_id": 376826,
    "total_money": 21848.01,
    "is_resell": 0,
    "consignee": "詹良琼",        // 收货人
    "phone": "13899666758",
    "province": "新疆维吾尔自治区",
    "city": "昌吉回族自治州",
    "area": "昌吉市",
    "address": "红星东路嘉禾欣居15号楼",
    "pay_img": "/upload/image/20260720/xxx.jpg",
    "pay_time": "2026-07-20T10:13:03+08:00",
    "buy_time": "2026-07-20T10:03:45+08:00",
    "confirm_time": "2026-07-20T10:22:09+08:00",
    "status": 2,                  // 1待确认 2已完成
    "created_at": "2026-07-20T10:03:45+08:00",
    "updated_at": "2026-07-20T10:22:09+08:00"
  }]
}
```

### GET /orders/:id

**使用页面**: 订单详情页

**出参**: 单条订单对象（同上）

### PUT /orders/:id/status

**使用页面**: 订单操作 → 确认收货/修改状态

**入参**:
```json
{
  "status": 2    // 1待确认 2已完成
}
```

### GET /exchange-orders

**使用页面**: 订单管理 → 兑换订单管理

> 老系统暂无兑换订单数据，返回空列表

**入参**: `?page=1&limit=20`

---

## 6. 商品管理

> 对应菜单：**商品管理 → 商品管理 + 商品分类 + 寄售商品**

### GET /goods

**使用页面**: 商品列表页

**入参**: `?page=1&limit=20`

**出参**:
```json
{
  "code": 0,
  "count": 3,
  "data": [{
    "id": 1,
    "category_id": 1,
    "title": "功能性食品家庭健康饮套装",
    "images": "/upload/image/20260430/xxx.jpg",
    "price": 3044.00,             // 售价
    "line_price": 3044.00,        // 原价(划线价)
    "stock_num": 10000,           // 库存
    "sales_volume": 0,            // 销量
    "content": "",                // 详情HTML
    "status": 1,                  // 0下架 1上架
    "created_at": "2025-04-01T17:42:30+08:00",
    "updated_at": "2026-04-30T16:00:41+08:00"
  }]
}
```

### POST /goods

**使用页面**: 新增商品弹窗

**入参**:
```json
{
  "category_id": 1,
  "title": "商品名称",
  "images": "/upload/xxx.jpg",
  "price": 99.99,
  "line_price": 199.99,
  "stock_num": 100,
  "content": "<p>详情HTML</p>",
  "status": 1
}
```

### PUT /goods/:id

**使用页面**: 编辑商品弹窗（只传要改的字段）

**入参**:
```json
{
  "title": "修改后的名称",
  "price": 88.88,
  "stock_num": 50
}
```

### PUT /goods/:id/stock

**使用页面**: 商品列表 → 快捷设置库存

**入参**:
```json
{
  "stock": 500
}
```
> 同时同步到 Redis，前端秒杀商品实时库存会更新

### GET /merchandises

**使用页面**: 商品管理 → 寄售商品列表（用户发布的转售商品，73161条）

**入参**: `?page=1&limit=20&status=0`（0待售 1已售）

**出参**:
```json
{
  "code": 0,
  "count": 73161,
  "data": [{
    "id": 378004,
    "user_id": 99226,
    "title": "功能性食品家庭健康饮套装",
    "image": "/upload/image/20260430/xxx.jpg",
    "price": 21848.01,
    "is_show": 1,                 // 前端显示 0否 1是
    "status": 0,                  // 0待售 1已售
    "created_at": "2026-07-19T23:43:09+08:00"
  }]
}
```

### PUT /merchandises/:id/status

**使用页面**: 寄售商品 → 上下架操作

**入参**:
```json
{
  "status": 1,      // 0待售 1已售
  "is_show": 0      // 可选，0隐藏 1显示
}
```

---

## 7. 提现管理

> 对应菜单：**提现管理 → 提现管理**

### GET /withdraws

**使用页面**: 提现列表页（742条，支持按状态筛选审批）

**入参**: `?page=1&limit=20&status=2`（1已打款 2待处理）

**出参**:
```json
{
  "code": 0,
  "count": 394,
  "data": [{
    "id": 8060,
    "transfer_no": "99549117845131335333",  // 转账单号
    "user_id": 94599,
    "cate": 4,                    // 2手动 4自动
    "account_type": 2,            // 1银行卡 2支付宝
    "account_id": 5245,           // 收款账户ID
    "money": 2000.00,             // 申请金额
    "handling_fee": 0.00,         // 手续费
    "actual_amount": 2000.00,     // 实到金额
    "status": 1,                  // 1已打款 2待处理
    "remark": "",
    "created_at": "2026-07-20T10:05:33+08:00",  // 申请时间
    "updated_at": "2026-07-20T10:09:33+08:00"   // 处理时间
  }]
}
```

### PUT /withdraws/:id/approve

**使用页面**: 提现列表 → 审批按钮（通过/拒绝）

**入参**:
```json
{
  "status": 1,              // 1=通过 3=拒绝
  "remark": "审批备注"
}
```

---

## 8. 财务明细

> 对应菜单：**数据明细 → 余额明细 + 优惠券明细 + 个人奖金明细 + 推广奖金明细**

### GET /logs/money

**使用页面**: 余额变动明细页

> 老系统暂无余额流水数据，返回空

**入参**: `?page=1&limit=20&user_id=xxx`

### GET /logs/coupon

**使用页面**: 优惠券明细页（41,433条）

**入参**: `?page=1&limit=20&user_id=xxx&type=1`

| 参数 | 说明 |
|------|------|
| type | 1=收益到账 2=提现扣减 |
| start_time / end_time | 时间范围筛选 |

**出参**:
```json
{
  "code": 0,
  "count": 41433,
  "data": [{
    "id": 375457,
    "user_id": 99842,
    "type": 1,                    // 1收入 2支出
    "money": 200.000,
    "before": 14.690,             // 变动前余额
    "after": 214.690,             // 变动后余额
    "memo": "提现退还",           // 备注
    "created_at": "2026-07-20T10:08:31+08:00"
  }]
}
```

### GET /logs/self-bonus

**使用页面**: 个人奖金明细页（139,056条）

**入参**: 同优惠券明细

**出参**: 结构同上，`memo` 多为"今日收益"

### GET /logs/share-bonus

**使用页面**: 推广奖金明细页（72,298条）

**入参**: 同优惠券明细

**出参**: 结构同上

---

## 9. 菜单规则

> 对应菜单：**权限管理 → 菜单管理**

### GET /rules

**使用页面**: 菜单管理列表（145条平铺）

### GET /rules/tree

**使用页面**: 左侧导航菜单渲染 + 菜单管理树形图

**出参**:
```json
{
  "code": 0,
  "data": [{
    "id": 7,
    "title": "用户管理",
    "icon": "layui-icon layui-icon-username",
    "key": "user",
    "pid": 0,                     // 0=顶级菜单
    "href": "",
    "type": 0,                    // 0菜单组(可折叠) 1菜单项(页面)
    "weight": 800,                // 排序权重
    "children": [{
      "id": 8,
      "title": "用户管理",
      "icon": "",
      "key": "plugin\\admin\\app\\controller\\UserController",
      "pid": 7,
      "href": "/app/admin/user/index",   // 前端路由
      "type": 1,
      "weight": 800
    }]
  }]
}
```

---

## 10. 权限管理

> 对应菜单：**权限管理 → 账户管理 + 角色管理**

### GET /admins

**使用页面**: 管理员账户列表

### POST /admins

**使用页面**: 新增管理员弹窗

**入参**:
```json
{
  "username": "newadmin",
  "password": "123456",     // 最少6位
  "nickname": "新管理员",
  "roles": "2"              // 角色ID，逗号分隔
}
```

### GET /roles

**使用页面**: 角色列表

**出参**:
```json
{
  "code": 0,
  "data": [{
    "id": 1,
    "name": "超级管理员",
    "rules": "*",               // 权限规则，*表示全部
    "status": 1
  }]
}
```

### POST /roles

**使用页面**: 新增角色弹窗

**入参**:
```json
{
  "name": "运营角色",
  "rules": "[1,2,3,4,5]",   // 菜单规则ID数组(JSON字符串)
  "status": 1
}
```

### PUT /roles/:id

**使用页面**: 编辑角色权限

**入参**:
```json
{
  "name": "运营角色(改)",
  "rules": "[1,2,3,4,5,6]",
  "status": 1
}
```

---

## 11. 内容管理

> 对应菜单：**商城设置 → 首页轮播图 + 首页广告图**

### GET /banners

**使用页面**: 轮播图列表

### POST /banners

**使用页面**: 新增轮播图

**入参**:
```json
{
  "title": "活动banner",
  "image": "/upload/banner.jpg",
  "url": "/pages/promo",    // 点击跳转链接
  "sort": 1,                // 排序(越大越前)
  "status": 1               // 0隐藏 1显示
}
```

### PUT /banners/:id

**入参**: 只传要修改的字段

### GET /ads

**使用页面**: 广告图列表

### POST /ads

**入参**: 同 banner

### PUT /ads/:id

**入参**: 同 banner

---

## 12. 秒杀管理

> 新增功能，老系统无对应菜单

### GET /flash-sale/events

**使用页面**: 秒杀活动管理页

**出参**:
```json
{
  "code": 0,
  "data": [{
    "id": 1,
    "product_id": 1,
    "stock": 100,               // 秒杀库存
    "price": 1999.00,           // 秒杀价
    "start_time": "2026-07-21T10:00:00+08:00",
    "end_time": "2026-07-21T10:30:00+08:00",
    "max_per_user": 1,          // 每人限购
    "status": 0                 // 0未开始 1进行中 2已结束
  }]
}
```

### POST /flash-sale/events

**使用页面**: 创建/编辑秒杀活动（后台发布商品到秒杀专区）

**入参**:
```json
{
  "product_id": 1,
  "stock": 100,
  "price": 1999.00,
  "start_time": "2026-07-21T10:00:00+08:00",
  "end_time": "2026-07-21T10:30:00+08:00",
  "max_per_user": 1,
  "status": 0
}
```
> 创建成功后自动同步库存到 Redis，C端实时可见

---

## 附录

### 错误码速查

| code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未登录/Token过期 |
| 403 | 无权限 |
| 1001 | 账号已存在 |
| 1002 | 用户不存在 |
| 1003 | 密码错误 |
| 1004 | 账号已冻结 |
| 2001 | 库存不足 |
| 2002 | 已售罄 |
| 2003 | 超出限购 |
| 2004 | 已购买过 |
| 2006 | 不在秒杀时段 |
| 3001 | 订单不存在 |
| 4001 | 提现不存在 |
| 4002 | 已审批过 |

### 分页说明

所有列表接口统一分页格式：
- 请求: `?page=1&limit=20`
- 响应: `{"code":0,"count":总条数,"data":[...]}`
- `count` 用于前端计算总页数

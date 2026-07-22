# C端 API 接口文档

> **Base URL**: `http://localhost:8080/api/v1/front`  
> **认证方式**: `Authorization: Bearer {token}`（登录后所有接口需要）  
> **响应格式**: `{"code":0,"msg":"ok","data":...}`

---

## 1. 注册

### POST /auth/register

**入参**:
```json
{
  "username": "13800000001",    // 必填，手机号
  "password": "123456",         // 必填，最少6位
  "nickname": "张三",           // 选填
  "mobile": "13800000001",     // 必填
  "invite_code": "fwpk5m"     // 选填，推荐人推广码，填写后自动绑定上级关系
}
```

**出参**:
```json
{
  "code": 0,
  "data": {
    "user_id": 99935,
    "username": "13800000001"
  }
}
```

---

## 2. 登录

### POST /auth/login

**入参**:
```json
{
  "username": "13800000001",    // 手机号
  "password": "123456"
}
```

**出参**:
```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOi...",
    "user_id": 99935,
    "username": "13800000001",
    "nickname": "张三"
  }
}
```

---

## 3. 秒杀（核心）

### GET /flash-sale/time

**说明**: 查询秒杀时间窗口，前端据此显示/隐藏秒杀入口

**无需入参**

**出参**:
```json
{
  "code": 0,
  "data": {
    "is_open": false,                       // 当前是否开放秒杀
    "is_weekday": true,                     // 是否工作日
    "in_window": false,                     // 是否在10:00-10:30窗口内
    "start_time": "2026-07-21 10:00:00",   // 今日开始时间
    "end_time": "2026-07-21 10:30:00",     // 今日结束时间
    "weekday_rule": "周一至周五",
    "time_rule": "10:00-10:30",
    "server_time": "2026-07-20 22:30:00"   // 服务器当前时间
  }
}
```

**前端使用**: 用 `is_open` 控制秒杀入口显示/隐藏。`is_open=false` 时按钮置灰或显示"秒杀未开放"。

### GET /flash-sale/products

**说明**: 秒杀商品列表（登录后）

**出参**:
```json
{
  "code": 0,
  "data": {
    "products": [{
      "id": 1,
      "title": "功能性食品家庭健康饮套装",
      "image": "/upload/image/20260430/xxx.jpg",
      "price": 1999.00,            // 秒杀价
      "origin_price": 3044.00,     // 原价
      "stock": 97,                 // 剩余库存（Redis实时）
      "max_per_user": 1,           // 每人限购
      "status": 1,                 // 0未开始 1进行中 2已售罄 3已结束
      "start_time": "2026-07-21 10:00:00",
      "end_time": "2026-07-21 10:30:00"
    }],
    "is_open": true,
    "time_info": { /* 同 flash-sale/time 返回 */ }
  }
}
```

### POST /flash-sale/buy 🔥

**说明**: 抢购接口，全局限流1000次/秒 + 用户级秒杀时间段内才可调用

**入参**:
```json
{
  "product_id": 1
}
```

**成功响应**:
```json
{
  "code": 0,
  "data": {
    "msg": "抢购成功！请等待系统确认订单"
  }
}
```

**失败响应**:

| 场景 | 响应 |
|------|------|
| 不在秒杀时间 | `{"code":2006,"msg":"不在秒杀时间段(工作日10:00-10:30)"}` |
| 已售罄 | `{"code":2002,"msg":"已售罄"}` |
| 已购买过 | `{"code":2004,"msg":"已购买过该商品"}` |
| 超出限购 | `{"code":2003,"msg":"超出限购数量"}` |
| 未登录 | `{"code":401,"msg":"请先登录"}` |

---

## 4. 商品

### GET /products

**说明**: 商品列表（分页）

**入参**: `?page=1&limit=10`

### GET /products/:id

**说明**: 商品详情，含Redis实时库存

**出参**:
```json
{
  "code": 0,
  "data": {
    "product": { /* 商品对象 */ },
    "stock": 97        // Redis实时库存
  }
}
```

---

## 5. 订单

### GET /orders

**说明**: 我的订单（登录用户）

**入参**: `?page=1&limit=10`

### GET /orders/:id

**说明**: 订单详情（只能查看自己的订单）

---

## 6. 用户

### GET /user/profile

**说明**: 个人信息 + 钱包

**出参**:
```json
{
  "code": 0,
  "data": {
    "user": { /* 用户对象 */ },
    "wallet": {
      "money": 0.000,           // 余额
      "coupon": 0.000,          // 优惠券
      "self_bonus": 0.000,      // 个人奖金
      "share_bonus": 0.000,     // 推广奖金
      "score": 0,               // 积分
      "poor": 0.00              // 待还款
    }
  }
}
```

### GET /user/wallet

**说明**: 仅钱包余额

---

## 前端集成要点

### 秒杀入口显隐逻辑
```javascript
// 页面加载时调用
fetch('/api/v1/front/flash-sale/time')
  .then(r => r.json())
  .then(res => {
    if (res.data.is_open) {
      // 显示秒杀入口
      showFlashSaleEntry()
    } else {
      // 隐藏或置灰
      hideFlashSaleEntry()
    }
  })
```

### 抢购按钮
```javascript
async function buy(productId) {
  const resp = await fetch('/api/v1/front/flash-sale/buy', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    },
    body: JSON.stringify({ product_id: productId })
  })
  const result = await resp.json()
  if (result.code === 0) {
    alert('抢购成功！')
  } else {
    alert(result.msg)  // "已售罄" / "已购买过" / "不在秒杀时间"...
  }
}
```

### 认证流程
1. 登录接口获取 `token`
2. 所有后续请求 Header 带 `Authorization: Bearer {token}`
3. Token 过期返回 `code:401`，前端跳转登录页

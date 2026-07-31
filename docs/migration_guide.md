# 老系统数据迁移完整流程

## 一、爬取老系统全量数据

### 1.1 准备

- 老系统域名：`api.srdsmgs.com`（API）、`www.srdsmgs.com`（前台）
- 获取老系统管理后台登录后的 Cookie（PHPSID=xxx）
- 确认 `tools/old_system_migration/apis.txt` 包含需要爬取的 21 个接口

### 1.2 执行全量爬取

```bash
cd tools/old_system_migration

# 编译爬虫
go build -o ./output/crawl.exe ./cmd/crawl/

# 全量翻页爬取（只读，100ms页间延迟不冲垮老系统）
./output/crawl.exe -fullsync \
  -api-list apis.txt \
  -cookie "PHPSID=xxx" \
  -output ./output_new
```

产出在 `output_new/data/` 下，每个接口一个 `*_FULL.json` 文件：
- `user_select_FULL.json` — 全部用户
- `order_select_FULL.json` — 全部订单
- `merchandise_select_FULL.json` — 全部寄售商品
- `withdraw_select_FULL.json` — 全部提现
- `coupon-log_select_FULL.json` — 优惠券流水
- `selfbonus-log_select_FULL.json` — 个人奖金流水
- `sharebonus-log_select_FULL.json` — 推广奖金流水
- 其他：分类/商品/管理员/角色/规则/轮播图等

---

## 二、构建用户树

### 2.1 递归查找粉丝

以指定用户为根，递归查找所有 `pid == 当前用户ID` 的子用户，直到无粉丝为止。

```go
func buildFanTree(allUsers, rootID) {
    visited := map[int64]bool{}
    var dfs func(pid)
    dfs = func(pid) {
        for id, u := range allUsers {
            if u.pid == pid && !visited[id] {
                visited[id] = true
                result.append(u)
                dfs(id) // 递归找粉丝的粉丝
            }
        }
    }
    dfs(rootID)
}
```

### 2.2 查找目标用户

在 `user_select_FULL.json` 中按昵称或手机号查找用户 ID。

---

## 三、定向迁移

### 3.1 密码重置

迁移完成后统一执行，所有用户密码 → `123456`：

```sql
UPDATE users SET password = MD5(CONCAT('123456', salt));
```

### 3.2 合同重置

老系统合同 PDF 不迁入新系统，合同状态统一为**未签订**。迁移脚本不写 `user_contracts` 表，不写 `users.contract` 字段：

```sql
DELETE FROM user_contracts;
UPDATE users SET contract = '';
```

迁入的用户需在新系统重新签订合同。

### 3.3 数据映射规则

迁移时需处理以下字段映射：

| 字段 | 老系统 | → | 新系统 |
|------|------|:--:|------|
| **用户等级 level** | `1` 普通用户 | → | `0` 普通用户 |
| | `2` 推荐人 | → | `1` 推荐人 |
| | `3` 店长 | → | `2` 店长 |
| **提现状态 status** | `0` 待审核 | → | `2` 待处理 |
| | `1` 已打款 | → | `1` 已通过 |
| | `2` 已驳回 | → | `3` 已驳回 |
| **货币类型** | `cate=2` | → | `currency_type="coupon"` |
| | `cate=4` | → | `currency_type="share_bonus"` |
| **图片路径** | `/uploads/xxx.png` | → | `/upload/image/xxx.png` |
| | `/upload/image/xxx.jpg` | → | 不变 |

### 3.4 订单寄卖状态

订单 `is_resell` 原样迁入，不做任何修改：

| is_resell | 老系统 | → | 新系统 |
|:--:|------|:--:|------|
| `0` | 待寄卖 | → | 待寄卖（买方仓库可点寄售） |
| `1` | 已寄卖 | → | 已寄卖（不显示寄售按钮） |

老系统 93,579 条订单中仅 10 条 `is_resell=0`，迁入后绝大部分已完成订单没有寄售按钮，这是源数据决定的，不是 bug。新系统新产生的订单在确认收款后 `is_resell=0`，用户可在买方仓库正常寄售。

### 3.6 迁移顺序

必须按依赖顺序执行：
1. users → user_wallets → user_contracts
2. merchandises（用户树内的用户）
3. orders（买方或卖方在用户树内）
4. withdraws → withdraw_accounts
5. coupon_logs / self_bonus_logs / share_bonus_logs

### 3.7 补全缺失商品

订单关联的寄售商品如果不在已迁移商品中，从老系统全量商品数据补入（临时禁用 `fk_merch_user` 外键）。

迁移后验证：
```sql
-- 订单商品覆盖率
SELECT COUNT(*) FROM orders WHERE merchandise_id NOT IN (SELECT id FROM merchandises);
-- 应为 0
```

---

## 四、下载老系统图片

### 4.1 提取图片列表

从 DB 的 merchandises / goods / users 表提取所有独立图片路径。

### 4.2 下载

脚本位于 `tools/old_system_migration/cmd/download_images/`：

```bash
cd tools/old_system_migration
go build -o ./output/download_images.exe ./cmd/download_images/
./output/download_images.exe \
  -cookie "PHPSID=xxx" \
  -base "https://api.srdsmgs.com" \
  -output ./output/downloaded_images
```

注意：`www.srdsmgs.com` 有防盗链（403），必须用 `api.srdsmgs.com` 域名下载。

### 4.3 规范化并复制到服务端

```bash
# 复制到服务端 upload 目录
cp -rn output/downloaded_images/upload/ ../../upload/
cp -rn output/downloaded_images/uploads/ ../../upload/  # 老系统双路径统一

# DB 路径标准化
UPDATE merchandises SET image = REPLACE(image, '/uploads/', '/upload/image/');
```

服务端自动兜底：本地不存在的图片请求返回 `./upload/placeholder.png`。

---

## 五、配置初始化

迁移完成后，需在管理后台 **系统配置** 中设置所有配置项，否则相关功能不生效（无硬编码默认值）：

### 秒杀规则（必须配）
- `flash_sale_days` — 开放日期，如 `1-5`
- `flash_sale_start` / `flash_sale_end` — 时段
- `priority_max_orders` / `priority_advance_minutes` — 优先用户配置

### 交易规则（必须配）
- `resell_rate` — 增值比例
- `static_income_rate` — 静态收益
- `order_reward_rate` — 抢单奖励（进 coupon 作为"今日收益"）
- `direct_referral_rate` — 直推收益
- `store_manager_rate` — 店长收益
- `listing_fee_rate` — 上架费
- `resell_deadline` — 寄卖截止时间
- `resell_product_id` / `flash_sale_seller_id` / `flash_sale_product`

### 功能开关
- `sms_verify` — 短信校验
- `coupon_withdraw_enable` / `referral_withdraw_enable` — 提现开关

没有配置 = 功能不生效 = 不扣钱也不分钱。

---

## 六、分佣机制对齐

| 分佣 | 配置 key | 入账目标 | 备注 |
|------|------|------|------|
| 静态收益 1% | `static_income_rate` | self_bonus（个人奖金） | |
| 抢单奖励 0.5% | `order_reward_rate` | **coupon（优惠券）** | 对应老系统"今日收益" |
| 直推收益 0.2% | `direct_referral_rate` | share_bonus（推广奖金） | 给直接上级 |
| 店长收益 1% | `store_manager_rate` | share_bonus（推广奖金） | 沿 PID 链找 level≥2 |

老系统"今日收益" = `order_reward_rate` 写入 coupon，这是用户付上架费的来源。

---

## 七、完整链路

```
1. 爬老系统（fullsync） → output_new/data/*.json
        │
2. 构建用户树（粉丝递归） → 确定迁移范围
        │
3. 定向迁移脚本（仅迁用户数据，不迁：系统配置/商品分类/商品模板/管理员/角色/菜单）
   ├─ 用户+钱包+合同（含 level/status/currency 映射）
   ├─ 订单（买方或卖方在树内即纳入）
   ├─ 寄售商品（树内用户 + 订单关联补全）
   ├─ 提现+收款账户
   └─ 财务日志
        │
4. 密码重置 + 合同清空
	├─ UPDATE users SET password = MD5(CONCAT('123456', salt))
	├─ DELETE FROM user_contracts
	└─ UPDATE users SET contract = ''
        │
5. 下载图片 → api.srdsmgs.com → 规范化路径 → 复制到 upload/，补充寄卖商品图
        │
6. 管理端配配置 → 秒杀/交易/开关
        │
7. 验证
   ├─ 订单商品覆盖率 = 100%
   ├─ 用户可登录（密码123456）
   ├─ 商品图正常显示
   ├─ 提现状态/货币类型正确
   └─ 新建商品→购买→付款→确认→寄卖 走通
```

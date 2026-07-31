# 老系统爬取与导入规则（严格遵守，禁止自作主张）

## 一、爬取

### 数据查询注意
老系统 JSON 格式为 `"id": 97872,`（冒号后有空格），直接搜 `"id":97872` 会查不到。
查用户先用手机号搜：`grep "17812059965" xxx_user_select_FULL.json`
确认存在后再定位具体字段。

### 命令
```bash
cd E:/Commercial_Transactions_Service/tools/old_system_migration
go build -o ./output/crawl.exe ./cmd/crawl/
./output/crawl.exe -fullsync \
  -api-list apis.txt \
  -cookie "PHPSID=xxx" \
  -output ./output_new2
```

### 规则
- 爬虫**只管爬**，不动数据库
- 产出在 `output_new2/data/` 下，全是 JSON 文件
- 爬完告知用户，等用户指令再下一步

---

## 二、导入

### 禁止事项（绝对不做）
1. **不建占位用户**：树外卖家不存在就不存在，关外键即可
2. **不导多余商品**：仅导订单关联的那一条商品，status=1（已售），不在寄售池
3. **不导已寄卖订单**：只导 `is_resell=0` 的
4. **不导其他用户**：用户说导谁就导谁，一个不多
5. **不保留老密码**：导入后必须重置为 `123456`
6. **不保留老合同**：导入后清空 `user_contracts` 和 `users.contract`

### 导入步骤（逐步执行，做完一步说一步）

#### 第1步：导用户
```sql
SET FOREIGN_KEY_CHECKS=0;
INSERT INTO users (id,username,nickname,mobile,password,salt,...) VALUES(...);
SET FOREIGN_KEY_CHECKS=1;
```
- 字段一一对应老系统
- 如果列数对不上，先 `DESCRIBE users` 确认

#### 第2步：导钱包
```sql
INSERT INTO user_wallets (user_id,money,coupon,self_bonus,share_bonus,...) VALUES(...);
```

#### 第3步：导订单
- **只导 `is_resell=0`（未寄卖）的**
- **只导该用户是买家的单子**
- 老系统 `is_resell=0` 就是未寄卖，直接原值写入
- 树外卖家没有用户记录，关外键处理，不建占位

#### 第4步：导订单关联的商品
- 查订单的 `merchandise_id`，从老系统爬到的商品 JSON 找到对应商品
- 导入该商品（status=1 已售，不在寄售池）
- 把订单的 `merchandise_id` 更新为正确值

#### 第5步：导提现

字段映射：
| 老系统字段 | 新系统字段 | 处理 |
|-----------|-----------|------|
| transfer_no | transfer_no | 直接写入 |
| user_id | user_id | 直接写入 |
| cate | cate + currency_type | cate=2→coupon, cate=4→share_bonus |
| account_type | account_type | 1=银行卡, 2=支付宝 |
| account_id | account_id | 直接写入 |
| money | money | 直接写入 |
| handling_fee | handling_fee | 直接写入 |
| actual_amount | actual_amount | 直接写入 |
| status | status | 老0→新2(待处理), 老1→新1(已通过), 老2→新3(已驳回) |
| remark | remark | 直接写入 |
| created_at | created_at | 保留原始时间，空值用 NOW() |
| account_info(JSON) | withdraw_accounts 表 | 解析后插入独立表 |

**账户信息**：老系统 `account_info` 是 JSON 字符串，解析后插入 `withdraw_accounts` 表。字段映射：
- `account_info.id` → `withdraw_accounts.id`
- `account_info.user_id` → `withdraw_accounts.user_id`
- `account_info.username` → `withdraw_accounts.username`
- `account_info.account` → `withdraw_accounts.account`
- 提现记录的 `account_type` → `withdraw_accounts.account_type`（1=银行卡，2=支付宝）
- `account_info.qrcode` → `withdraw_accounts.qrcode`
- `account_info.phone` → `withdraw_accounts.phone`

**注意**：`account_info` 内包含 `"user_id"`，与提现记录的 `user_id` 冲突，JSON 解析时不能用字符串匹配，必须用 `encoding/json` 标准库。

**排序**：管理端提现列表按 `created_at DESC, id DESC`，最新申请在前。

#### 第6步：导财务明细

老系统 JSON 文件名 → 新系统表名：
| 老系统文件 | 新系统表 | 注意 |
|-----------|---------|------|
| coupon-log | coupon_logs | 一致 |
| selfbonus-log | self_bonus_logs | 老系统无下划线 |
| sharebonus-log | share_bonus_logs | 老系统无下划线 |

字段一一对应：user_id, type(1收入/2支出), money, before, after, memo, created_at。

用 `tools/gen_sql.go` 将 JSON 转为 INSERT，管道给 MySQL 执行：`go run tools/gen_sql.go | mysql ...`

**排序**：C端明细接口按 `created_at DESC, id DESC`，最新记录在前（wallet_logs.go queryLogs）。

**本次导入量**：coupon 12455, self_bonus 16449, share_bonus 15508。

#### 第7步：密码重置（必须）
```sql
UPDATE users SET password = MD5(CONCAT('123456', salt)) WHERE id = xxx;
```

#### 第8步：合同清空（必须）
```sql
DELETE FROM user_contracts WHERE user_id = xxx;
UPDATE users SET contract = '' WHERE id = xxx;
```

### 完整流程

```
1. 爬虫（只读不写）
2. 导用户 → 导钱包 → 导订单 → 导商品 → 导提现 → 导明细
3. 密码重置 + 合同清空
```

**不迁移的**：系统配置、商品分类、商品模板、管理员、角色、菜单。

---

## 三、财务明细导入

老系统 JSON 文件名为 `selfbonus-log`、`sharebonus-log`（无下划线），对应新系统表 `self_bonus_logs`、`share_bonus_logs`。

使用 `tools/gen_sql.go` 将 JSON 转为 INSERT 语句（不连数据库），管道给 MySQL 执行写入：

```bash
go run tools/gen_sql.go | mysql -h127.0.0.1 -uroot -proot flash_sale
```

**原则：Go 脚本只做 JSON→SQL 的格式转换，不连 DB；实际写入由 MySQL CLI 执行。**

### C端展示排序
导入后 C端明细接口需按 `created_at DESC` 排序，最新记录在最前面。已在 `wallet_logs.go` 的 `queryLogs` 中实现。

### 本次导入结果（用户 97872 李雷）

| 数据 | 条数 | 备注 |
|------|------|------|
| 用户 | 1 | 97872 李雷 |
| 钱包 | 1 | money=0, coupon=6455.32, self=20716.334, share=11538.715 |
| 订单 | 1 | 仅 is_resell=0 的那条 (408554) |
| 提现 | 0 | - |
| 优惠券明细 | 93 | - |
| 个人奖金明细 | 95 | - |
| 推广奖金明细 | 202 | - |

---

## 四、不迁移的数据
以下数据**保留新系统自己的**，不从老系统爬取导入：
- 系统配置（system_configs）
- 商品分类（categories）
- 商品模板（goods）
- 管理员（admin_users）
- 角色（roles）
- 菜单（rules）
- 轮播图/广告图（banners/ads）

---

## 五、用户数据导出表格式

```
导出文件名: 用户下单统计表{当天日期}.xlsx

表头: 昵称 | {月-日}卖货 | {月-日}买货 | 差额 | 推广人ID | 推广人
```

---

## 六、核心原则
1. **用户没说的，一个字都别多做**
2. **遇到外键报错先问，别自己建占位数据**
3. **每一步做完汇报结果，再等指令**
4. **数据是适配系统的，不是系统适配数据的**

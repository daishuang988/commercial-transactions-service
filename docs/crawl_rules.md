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
- **结算后爬才完整**：老系统 23:59 结算（约 00:05 落库），优惠券明细等在结算后再爬，否则当天"今日收益"缺失
- **最终爬取前必须重爬用户增量、重算粉丝树**：树算完后可能还有新用户注册入树（本次漏了 100507 付洪亮，23:19 注册），爬完用户后重新跑一遍 fan_tree_write 核对人数

---

## 二、导入

### 禁止事项（绝对不做）
1. **不建占位用户**：树外卖家/买家不存在就不存在，关外键即可（本次树外 16,769 卖家 / 16,071 买家均未建用户）
2. **不导范围外用户**：用户说导谁就导谁（本次为 94694 全粉丝树 655 人），一个不多
3. **不保留老密码**：导入时必须重置为 `123456`，SQL 生成阶段直接写 `MD5(CONCAT('123456', salt))`
4. **不保留老合同**：清空 `user_contracts` 和 `users.contract`
5. **Go 脚本不连数据库**：只做 JSON→SQL 转换，实际写入由 MySQL CLI 执行

### 导入范围（每次由用户拍板，别自作主张）
- 订单是否全量（含 `is_resell=1`）、寄售池商品是否导入、商品状态怎么处理，以用户当场确认的为准
- **本次（94694 粉丝树）用户拍板**：订单全量（买方或卖方在树内即导，含已寄卖，37,695 条）；寄售池商品全量原状态导入（21,961 件树内商品）；订单关联的树外商品补导入用于订单展示（15,047 件，按新系统语义 status=1 已售）

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
- 范围由用户拍板（本次全量：买方或卖方在树内即导，含 `is_resell=1`）
- **关键防重**：老系统已结算过的已完成订单（status=2），导入时 `coupon_settled` 直接写 **1**——新系统每天 23:59 结算 `status=2 AND coupon_settled=0` 的订单，不置 1 会把老系统已发的"今日收益"再发一遍
- 老系统 `merchandise_id` 是旧代 ID：构建 old_id→new_id 映射表，560 单重映射；老系统已删商品的 1,660 单保留悬空引用（管理端商品信息为空属老系统原状，正常）

#### 第4步：导寄售商品
- 树内用户的寄售池商品按老系统**原状态**导入（待售就待售）
- 订单关联的树外卖家商品一并导入，供订单详情显示（LEFT JOIN 不筛 is_show）
- **老系统数据不一致的坑**：15,062/15,063 个已完成订单对应的商品在老系统仍是待售（status=0）——老数据不可全信，此类"已被订单引用"的商品按**新系统语义**改为 `status=1（已售） + is_show=1`（新系统下单成功即把商品 0→1，is_show 不变）
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

## 六、94694 全粉丝树导入实录（2026-08-14）

首次大规模实际执行：94694 袁小华（13716057283）粉丝树 **655 人**全量迁入通瑞辰生产库（123.57.68.22 flash_sale）。

### 工具链（怎么做）

```bash
cd tools/old_system_migration

# 1. 爬全量（7 个接口：user/order/merchandise/withdraw/account + 3 类流水）
./output/crawl.exe -fullsync -api-list apis.txt -cookie "PHPSID=xxx" -output ./output_new2

# 2. 算粉丝树（BFS 逐层找 pid 关系，输出排序后的 ID 列表文件）
go build -o fan_tree_write.exe fan_tree_write.go
./fan_tree_write.exe output_new2/data/user_select_FULL.json 94694 fan_tree_ids.txt
# → 655 人，层分布 12/28/99/215/191/91/16/2（根+7层）

# 3. 结算后（00:05 后）重爬一轮核对增量，重算树防漏新注册

# 4. JSON→SQL（不连库，生成 9 个文件，每文件头 SET FOREIGN_KEY_CHECKS=0，09 末尾恢复）
go build -o import_tree.exe import_tree.go
./import_tree.exe fan_tree_ids.txt output_new2/data output_import
# 01_users 02_user_wallets 03_merchandises 04_orders 05_withdraw_accounts
# 06_withdraws 07_self_bonus_logs 08_share_bonus_logs 09_coupon_logs

# 5. 按编号顺序执行（MySQL CLI）
mysql -h127.0.0.1 -uroot -p flash_sale < output_import/01_users.sql  # ... 依次到 09

# 6. 验证（见下方清单）
```

`output_import/`（含敏感数据的 SQL）已加 .gitignore，不入库。服务器备份在 `/opt/app/import_94694/`。

### 导入结果

| 数据 | 条数 | 备注 |
|------|------|------|
| 用户 | 655 | 密码全部 MD5('123456'+salt)，contract 清空 |
| 钱包 | 655 | 老系统 users 内联余额拆分到 user_wallets |
| 订单 | 37,695 | 37,691 已完成（coupon_settled=1 防重结算）+ 4 待确认 |
| 寄售商品 | 37,008 | 树内寄卖中 21,425 + 已售 15,583（树内 536 + 树外 15,047） |
| 提现 | 200 | 老状态 0/1/2→新 2/1/3；4 待处理 / 63 已打款 / 133 已驳回 |
| 收款账户 | 98 | 从提现 account_info JSON 解析，按 id 去重 |
| 优惠券流水 | 12,218 | coupon_logs |
| 个人奖金流水 | 21,623 | self_bonus_logs |
| 推广奖金流水 | 20,646 | share_bonus_logs |

### 决策反复（记录下来避免重复踩坑）

1. **树外卖家商品先删后恢复**：最初只导树内商品，用户指出"树外卖家的未售商品没意义"→ 删掉 15,047 件（备份 merch_outside_tree_backup_15047.sql）；随后用户发现"树内买家在买方仓库看不到这些订单的商品图"→ 恢复这 15,047 件；最后用户指出"都被买走了怎么还是未售+隐藏"→ 全部改为 status=1（已售）+ is_show=1。
   **最终定论**：订单引用的树外商品必须导入（订单详情 JOIN 需要），按新系统语义为"已售+显示"——寄卖池只筛 status=0，已售商品天然不污染售卖列表。
2. **老数据不可全信**：老系统 15,062 个已完成订单的商品仍是待售状态，导入时不能照搬，按新系统语义修正。
3. **防 23:59 重复结算**：老系统每天 23:59 结算，新系统也是——已完成订单导入时 coupon_settled=1，否则新系统当晚给买家重发一遍"今日收益"。

### 验证清单

- [ ] 各表条数本地与服务器完全一致
- [ ] 树内用户登录验证（123456）：13716057283（袁小华）、13699190269（付洪亮）均通过
- [ ] 新系统 23:59 结算跑完后，确认无老订单被重复结算（coupon_logs 无新增）
- [ ] 寄卖池只剩树内卖家商品（`SELECT COUNT(*) FROM merchandises m LEFT JOIN users u ON u.id=m.user_id WHERE m.status=0 AND u.id IS NULL` = 0）
- [ ] 订单详情能 JOIN 到商品信息（含树外卖家的单子）

---

## 七、核心原则
1. **用户没说的，一个字都别多做**
2. **遇到外键报错先问，别自己建占位数据**
3. **每一步做完汇报结果，再等指令**
4. **数据是适配系统的，不是系统适配数据的**
5. **老数据状态不可全信**：导入前先核对"数据之间是否自洽"（如订单已完成但商品仍待售），不一致时按新系统语义处理并告知用户
6. **动了生产数据先备份**：删除/修改前 dump 到服务器备份目录，留恢复余地

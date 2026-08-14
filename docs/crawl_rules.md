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
- **本次（94694 粉丝树）用户拍板**：订单全量（买方或卖方在树内即导，含已寄卖，37,695 条）；寄售池商品全量导入（21,961 件树内商品，⚠️ status 需反转映射，见第4步）；订单关联的树外商品补导入用于订单展示（15,047 件，按新系统语义 status=1 已售）

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
- **⚠️ 树内商品导入时必须反转 status**：老系统 status 语义与新系统**相反**——老 0=已售、1=未售；新 0=待售、1=已售。映射 `status_new = 1 - status_old`。2026-08-14 首轮按原值照搬导致全部反转（21,425 件已售商品在新系统变"待售"进入寄卖池，536 件真正未售的反而隐藏），当晚用 `UPDATE merchandises m JOIN users u ON u.id=m.user_id SET m.status = 1 - m.status` 修复（备份 merch_status_invert_backup_0814_0259.sql）
- 老系统"已售"判定**只看 status 字段，不看订单存在性**（2026-08-14 曾误以为"有订单即已售"改代码，被用户用反例纠正：285563 老系统无任何订单但显示已售，407942 老系统无订单但显示未售）
- 订单关联的树外卖家商品一并导入，供订单详情显示（LEFT JOIN 不筛 is_show），状态处理问用户（本次按用户要求置 status=1 已售，不做反转）
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
2. **老系统 status 语义相反**：老 0=已售、1=未售，新系统正好相反（0=待售、1=已售），且老系统判定**只看 status 不看订单**。首轮按原值导入导致全部反转；此后又误判"已售=有订单"改数据、改代码，被用户用 285563（无订单但老系统已售）/407942（无订单但老系统未售）两个反例纠正。**导入前必须问清字段语义，别假设两套系统字段含义相同**
3. **防 23:59 重复结算**：老系统每天 23:59 结算，新系统也是——已完成订单导入时 coupon_settled=1，否则新系统当晚给买家重发一遍"今日收益"。
4. **已售判定最终对齐方式＝数据反转，不改代码判定**：老系统只看 status 字段（语义相反），把导入数据的 status 反转（`status_new = 1 - status_old`）即可完全对齐，代码保持原生 status 过滤。验证口径：C端寄卖池 536、管理端待售 536 / 已售 36,472（树内 21,425 + 树外 15,047）。老系统全部用户未售 1,798 = 树内 536 + 树外未导入 1,262（树外未售商品按用户决定不导入）。
5. **卖方仓库寄售商品不可见 → mine=1 接口修复**：前端卖方仓库原来从寄卖池第一页前 50 条（id DESC）里客户端筛自己的商品，池内排名 50 之后的商品永远看不到（郭作华的 407258 排第 160），已售商品也不显示。修复：后端 `/merchandises` 加 `mine=1` 参数直接按当前用户过滤（不限 status，含已售），前端卖方仓库改调它。验证（郭作华 99990）：卖方仓库 = 16 笔订单 + 17 件寄售商品（1 寄售中 + 16 已售）= 33 条，与老系统该用户订单/商品数完全对齐。后端 dd823eb、前端 ce7984d。

### 老系统寄卖链知识（排查 405546 循环时发现）

- 老系统寄卖商品的 `old_id` = **源订单 ID**（不是上一代商品 ID）。例：407258.old_id=427397（郭作华买 405546 的那笔订单）。
- 循环模式：每日 10:00 自动抢购 → 约 15:30 自动寄卖（每轮按 resell_rate +2%）→ 次日再被抢。405546 已循环 3 轮：403751 →(订单 425998)→ 405546 →(订单 427397)→ 407258（寄卖中）。
- 商品 `status`（0 寄卖中 / 1 已售）与订单 `is_resell`（买家是否已寄卖，0 可点寄售 / 1 已寄卖）是两回事，勿混。

### 商品标准化（2026-08-14）

寄卖池 536 件 + 买方仓库未寄卖（is_resell=0）订单关联的 48 件，共 584 件商品图/标题统一改为「不锈钢锅+锌泉水杯实用套装」（图 /upload/image/20260814013409.jpg），备份 merch_img_title_backup_0814.sql。其余已售商品未动。

### 验证清单

- [ ] 各表条数本地与服务器完全一致
- [ ] 树内用户登录验证（123456）：13716057283（袁小华）、13699190269（付洪亮）均通过
- [ ] 新系统 23:59 结算跑完后，确认无老订单被重复结算（coupon_logs 无新增）
- [ ] 寄卖池只剩树内卖家商品（`SELECT COUNT(*) FROM merchandises m LEFT JOIN users u ON u.id=m.user_id WHERE m.status=0 AND u.id IS NULL` = 0）
- [ ] 订单详情能 JOIN 到商品信息（含树外卖家的单子）

---

## 八、重新爬取重导完整步骤（2026-08-15 执行）

老系统数据每天结算一次（23:59 结算、约 00:05 落库），**必须凌晨结算完成后爬取**，否则当天"今日收益"缺失。2026-08-14 曾测试产生脏数据（订单 429156/429157、商品 407944/407945、exchange_orders 18、407258 被改状态/标题），重导前必须清理。

### 第 0 步：清理上一轮数据（服务器 DB）

1. **先备份现状**到 /opt/app/import_94694/（动生产数据先备份）
2. 清测试脏数据：`DELETE FROM orders WHERE id IN (429156,429157)`；`DELETE FROM merchandises WHERE id IN (407944,407945)`；`DELETE FROM exchange_orders WHERE id=18`；407258 恢复 status=0、标题去掉"-测试"
3. 删除上一轮导入的树内数据（users 树内 655 人及其 wallets/merchandises/orders/withdraws/withdraw_accounts/三类流水/合同），关外键执行，范围与用户确认后删
4. 清 Redis 测试计数：`DEL flash:user:daily:100465:* flash:user:daily:99990:*` 及当天其他测试 key

### 第 1 步：爬取（凌晨结算后）

⚠️ 结算时间修正（8-15 实测）：老系统结算流水是 **23:46 开始、写到次日凌晨 01:43** 才写完（"00:05 落库"的说法不准）。爬前先查 `selfbonus-log` 最新"今日收益"条目时间，确认写完再爬，否则当天收益缺失。8-15 用户拍板在结算前就爬（接受 8-14 收益缺失）。

```bash
cd tools/old_system_migration
./output/crawl.exe -fullsync -api-list apis.txt -cookie "PHPSID=xxx" -output ./output_new3
```

爬虫只管爬不动库，爬完**重算粉丝树核对人数**（上次漏了 23:19 新注册的 100507 付洪亮）：

```bash
go build -o fan_tree_write.exe fan_tree_write.go
./fan_tree_write.exe output_new2/data/user_select_FULL.json 94694 fan_tree_ids.txt
```

爬完汇报，等用户指令。

### 第 2 步：JSON→SQL

```bash
go build -o import_tree.exe import_tree.go
./import_tree.exe fan_tree_ids.txt output_new2/data output_import
```

Go 脚本只做 JSON→SQL 转换**不连库**。生成 01_users 02_user_wallets 03_merchandises 04_orders 05_withdraw_accounts 06_withdraws 07_self_bonus_logs 08_share_bonus_logs 09_coupon_logs。SQL 生成阶段直接内置：密码 `MD5(CONCAT('123456', salt))`、清合同、**status 反转（status_new = 1 - status_old）**、已完成订单 coupon_settled=1、提现 0/1/2→2/1/3、cate 2/4→coupon/share_bonus、level 1/2/3→0/1/2、**订单范围＝买家在树内才导**（8-15 起内置，8-14 曾全量导入后再删 16,071 条）、树外卖家商品 status=1 一并导入（订单 JOIN 展示需要）。图片路径 /uploads/→/upload/image/ 未内置，导入后 `UPDATE merchandises SET image=REPLACE(image,'/uploads/','/upload/image/') WHERE image LIKE '/uploads/%'`（8-15 实际 10 条）。

### 第 3 步：导入（按编号顺序）

```bash
mysql -h127.0.0.1 -uroot -p flash_sale < output_import/01_users.sql  # ... 依次到 09
```

每文件头已含 SET FOREIGN_KEY_CHECKS=0。

### 第 4 步：导入后处理

1. **商品标准化**（重导后必须重做）：寄卖池 + 未寄卖(is_resell=0)订单关联的商品，图/标题统一为「不锈钢锅+锌泉水杯实用套装」（图 /upload/image/20260814013409.jpg，先备份）
2. **图片同步**（上次未做）：download_images.exe 下载老系统图片（用 api.srdsmgs.com，www 有防盗链 403），复制到服务器 upload 目录，DB 路径规范化

### 第 5 步：验证

- [ ] 各表条数本地与服务器一致
- [ ] 树内用户登录 123456（13716057283 袁小华、13699190269 付洪亮）
- [ ] 寄卖池 = 树内未售；管理端待售/已售数 = 老系统口径（老系统全部未售 = 树内未售 + 树外未导入）
- [ ] 23:59 结算跑完无重复结算（coupon_logs 无重复新增）
- [ ] 订单商品覆盖率（悬空引用 = 老系统已删商品，正常）
- [ ] 卖方仓库 mine=1 接口正常（后端已就绪，前端由前端同事负责）

### 吃过的亏（别吃第二次）

1. **status 语义相反**：老 0=已售/1=未售，必须反转导入；判定只看 status 不看订单
2. **不反转**= 21,425 件已售变待售进池、536 件未售被隐藏（首轮事故）
3. **已完成订单不置 coupon_settled=1** = 23:59 重复发"今日收益"
4. **结算前爬** = 当天今日收益缺失
5. **爬完不算粉丝树** = 漏新注册用户（100507）
6. **树外商品不导** = 订单详情商品信息为空（先删后恢复过一次）
7. **不重置密码/不清合同** = 老密码保留、老合同带入（违反规则）
8. **测试数据不清** = 残留脏订单/商品，重导前必须清+备份
9. **account_info 用字符串匹配解析** = 失败，必须 encoding/json
10. **Go 脚本连库** = 禁止，只 JSON→SQL，写入由 MySQL CLI 执行

### 2026-08-15 凌晨执行实录（全部通过）

- 00:20 爬取（19 分钟，output_new3，21 接口全）→ 重算粉丝树 **655→668**（+13 人全是 8-14 新注册，0 减少）
- 00:49 全库备份 backup_full_0815_重导前.sql → 清 12 表（含新系统测试注册用户 100508）→ SQL 生成后抽样核对：level 0 错、status 反转 0 错、提现 0 错、密码/合同/ coupon_settled 全对
- 00:53 执行 01-09 全 OK。结果：用户 668、钱包 668、订单 21,624（买家在树内）、商品 37,012（**池 540** + 已售 36,472）、提现 201、账户 99、流水 12,220/21,624/20,648
- 导入后：图片路径修正 10 条；商品标准化 546 件（池 540 + is_resell=0 关联 6）
- 验证全过：会重复结算订单 0、池内树外卖家 0、悬空商品 1,660（=老系统原状）、登录 123456 通过（13716057283/13699190269）、袁小华钱包与老系统 API 数值一致

---

## 七、核心原则
1. **用户没说的，一个字都别多做**
2. **遇到外键报错先问，别自己建占位数据**
3. **每一步做完汇报结果，再等指令**
4. **数据是适配系统的，不是系统适配数据的**
5. **老数据的业务语义可能与新系统不同**：两套系统的字段含义未必一致，要改老数据状态/含义前先跟用户确认，不要按新系统逻辑反推老数据"应该是"什么
6. **动了生产数据先备份**：删除/修改前 dump 到服务器备份目录，留恢复余地

# 部署上线进度追踪

> 最后更新：2026-08-04

---

## 一、服务器信息

| 项目 | 配置 |
|------|------|
| ECS | g9i.xlarge 4核16G，华北2（北京），Ubuntu 22.04 |
| 公网 IP | `123.57.68.22` |
| 数据盘 | 100G ESSD 挂载 `/data` |
| SSH | 仅密钥登录，禁止密码 |
| 防火墙 | UFW，仅开放 22/80/443 |
| 年费 | ¥4,723（含磁盘） |

### 服务账号密码

| 服务 | 账号 | 密码 |
|------|------|------|
| SSH | root | 密钥登录 `-i .ssh_key_deploy`（原密码已禁用） |
| MySQL | root | `trc2024!@#Mysql` |
| Redis | — | `trc2024!@#Redis` |

### 服务器关键路径

| 用途 | 路径 |
|------|------|
| Go 后端 | `/opt/app/server` |
| 配置文件 | `/opt/app/config/config.yaml` |
| MySQL 数据 | `/data/mysql/` |
| 每日备份 | `/data/backup/` |
| 用户上传 | `/opt/app/upload/` |
| 移动端前端 | `/opt/app/h5-mobile/` |
| 管理端前端 | `/opt/app/h5-admin/` |
| SSL 证书 | `/etc/nginx/ssl/` |
| Go 日志 | `/opt/app/app.log` + `error.log` |
| systemd 服务 | `/etc/systemd/system/commercial.service` |
| Nginx 配置 | `/etc/nginx/sites-available/tongruichen` |
| 备份脚本 | `/opt/app/backup.sh`（每天凌晨 3:00） |

---

## 二、域名 & 证书

| 域名 | 用途 | 状态 |
|------|------|:--:|
| `tongruichen.com` | 主域名 | ✅ 已购买，¥80/年 |
| `m.tongruichen.com` | 移动端 | ✅ 已购买 SSL 证书，有效期 3 个月 |
| `admin.tongruichen.com` | 管理端 | ✅ 已购买 SSL 证书，有效期 3 个月 |

### SSL 证书续期
- 免费 DV 证书，3 个月有效期
- 到期前重新申请 → 替换 `/etc/nginx/ssl/` 下 `.pem` + `.key`
- `nginx -t && systemctl reload nginx` → 不停机
- DNS 验证在阿里云控制台操作

---

## 三、ICP 备案

| 步骤 | 状态 |
|------|:--:|
| 域名实名 | ✅ 已通过 |
| ICP 提交 | ✅ 已提交 |
| 管局审核 | ⏳ 等待中（20-30 工作日） |

- 备案类型：企业
- 网站名称：**通瑞辰商业交易服务平台**
- 域名：`tongruichen.com`
- 备案通过后 DNS 加 A 记录即可正式上线

---

## 四、部署服务状态

| 服务 | 状态 | 说明 |
|------|:--:|------|
| MySQL 8.0.46 | ✅ | Buffer Pool 8G，数据在 `/data/mysql` |
| Redis 6.0.16 | ✅ | maxmemory 2g，AOF 持久化 |
| Nginx 1.18.0 | ✅ | HTTP+HTTPS，双域名，自动跳转 |
| Go 后端 | ✅ | systemd 守护，`Restart=always` |
| 前端-移动端 | ✅ | `http://123.57.68.22` |
| 前端-管理端 | ✅ | 配 hosts 后 `https://admin.tongruichen.com` |
| 定时备份 | ✅ | 每天凌晨 3:00 mysqldump，保留 7 天 |
| 安全加固 | ✅ | SSH 密钥、UFW 防火墙 |

---

## 五、短信服务

| 配置 | 值 |
|------|------|
| 签名 | 河北雄安通瑞辰商贸（审核中） |
| 模板 CODE | `SMS_511510148` |
| AccessKey ID | `LTAI5t8SVRkJoaKtF7obDP9H` |
| 单价 | ¥0.045/条 |

### 逻辑
- `sms_verify=0`：降级模式，验证码固定 `"1234"`
- `sms_verify=1`：真实短信，生成 6 位随机码，阿里云下发
- 前端无需改动，响应不返回 `code` 时用户看手机短信

---

## 六、费用总览

| 类别 | 年费 |
|------|------|
| ECS（含磁盘） | ¥4,723 |
| 域名 | ¥80 |
| 短信 10,000 条 | ¥400 |
| 流量 | ~¥100 |
| SSL | ¥0 |
| **合计** | **≈ ¥5,300** |
| 预算 | ¥7,000 |
| 剩余 | ¥1,700 |

短信+流量月费估算（1000人）：**约 ¥16/月**

---

## 七、上线前修复状态

### ✅ 已修复（15 项）
SQL注入参数化、JWT Secret 环境变量、CORS 可配置、MD5→bcrypt、限流 4 路由、money_logs、优雅关闭、config 环境变量、adminLogin 去重、roles status、DELETE 端点、健康检查、手机号注册支持

### ⏳ 待处理（3 项，🔵 优化）
请求追踪 Request ID、文件上传类型校验、管理端审计日志

---

## 八、上线后要做

| 事项 | 说明 |
|------|------|
| DNS 加 A 记录 | ICP 通过后，`m` 和 `admin` 指向 ECS IP |
| 短信签名审核 | 通过后启动服务测试真实下发 |
| SSL 证书续期 | 每 3 个月续一次 |
| 老系统数据迁移 | 跑 `scripts/remigrate_all.go` 导入全量数据 |

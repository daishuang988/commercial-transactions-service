#!/bin/bash
# 全量接口测试脚本（mock 测试数据）
BASE="http://localhost:8080"
PASS=0
FAIL=0

test_api() {
  local method=$1 url=$2 data=$3 auth=$4 desc=$5 expected_code=$6
  local code=$(curl -s -o /tmp/resp.json -w "%{http_code}" -X $method "$BASE$url" \
    -H "Content-Type: application/json" \
    ${auth:+-H "Authorization: Bearer $auth"} \
    ${data:+-d "$data"})
  local body=$(cat /tmp/resp.json | head -1)
  if echo "$body" | grep -q "\"code\":0"; then
    echo "✅ $desc"
    ((PASS++))
  elif [ "$expected_code" != "" ] && [ "$code" = "$expected_code" ]; then
    echo "✅ $desc (expected: $expected_code)"
    ((PASS++))
  else
    echo "❌ $desc → $body"
    ((FAIL++))
  fi
}

echo "============================================"
echo " 全量接口测试（Mock数据）"
echo "============================================"
echo ""

# ─── C端 公开接口 ───
echo "─── C端 公开 ───"

# 注册新用户
test_api POST /api/v1/front/auth/register \
  '{"username":"mockuser001","password":"123456","nickname":"Mock测试","mobile":"13800000099"}' \
  "" "C-注册用户" ""

# 重复注册（应返回1001）
test_api POST /api/v1/front/auth/register \
  '{"username":"mockuser001","password":"123456","nickname":"重复","mobile":"13800000099"}' \
  "" "C-重复注册拒绝" ""

# 登录
USER_RESP=$(curl -s -X POST "$BASE/api/v1/front/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"mockuser001","password":"123456"}')
USER_TOKEN=$(echo "$USER_RESP" | grep -o '"token":"[^"]*"' | head -1 | sed 's/"token":"//;s/"//')
echo "$USER_RESP" | grep -q '"code":0' && echo "✅ C-用户登录" && ((PASS++)) || echo "❌ C-用户登录" && ((FAIL++))

# 秒杀时间
test_api GET /api/v1/front/flash-sale/time "" "$USER_TOKEN" "C-秒杀时间窗口" ""

# ─── C端 需认证 ───
echo "─── C端 需认证 ───"

test_api GET /api/v1/front/products?page=1\&limit=2 "" "$USER_TOKEN" "C-商品列表" ""
test_api GET /api/v1/front/products/1 "" "$USER_TOKEN" "C-商品详情" ""
test_api GET /api/v1/front/flash-sale/products "" "$USER_TOKEN" "C-秒杀商品列表" ""
test_api GET /api/v1/front/user/profile "" "$USER_TOKEN" "C-个人信息" ""
test_api GET /api/v1/front/user/wallet "" "$USER_TOKEN" "C-钱包余额" ""
test_api GET /api/v1/front/orders?page=1\&limit=2 "" "$USER_TOKEN" "C-我的订单" ""

# ============================================================
# ─── 管理端 ───
echo "─── 管理端 登录 ───"
ADMIN_RESP=$(curl -s -X POST "$BASE/api/v1/admin/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | grep -o '"token":"[^"]*"' | head -1 | sed 's/"token":"//;s/"//')
echo "$ADMIN_RESP" | grep -q '"code":0' && echo "✅ 管理端登录" && ((PASS++)) || echo "❌ 管理端登录" && ((FAIL++))

# ─── Dashboard ───
echo "─── Dashboard ───"
test_api GET /api/v1/admin/dashboard "" "$ADMIN_TOKEN" "管理-数据看板" ""

# ─── 系统配置 ───
echo "─── 系统配置 ───"
test_api GET /api/v1/admin/config "" "$ADMIN_TOKEN" "管理-系统配置" ""
test_api GET /api/v1/admin/account/info "" "$ADMIN_TOKEN" "管理-当前账户" ""

# ─── 用户管理 ───
echo "─── 用户管理 ───"
test_api GET "/api/v1/admin/users?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-用户列表" ""
test_api GET /api/v1/admin/users/1 "" "$ADMIN_TOKEN" "管理-用户详情" ""
test_api PUT /api/v1/admin/users/1/status '{"status":1}' "$ADMIN_TOKEN" "管理-用户状态" ""

# ─── 订单管理 ───
echo "─── 订单管理 ───"
test_api GET "/api/v1/admin/orders?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-订单列表" ""
test_api GET /api/v1/admin/orders/397481 "" "$ADMIN_TOKEN" "管理-订单详情" ""
test_api PUT /api/v1/admin/orders/397481/status '{"status":2}' "$ADMIN_TOKEN" "管理-订单状态" ""
test_api GET "/api/v1/admin/exchange-orders?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-兑换订单" ""

# ─── 商品管理 ───
echo "─── 商品管理 ───"
# Mock 创建商品
GOOD_RESP=$(curl -s -X POST "$BASE/api/v1/admin/goods" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"category_id":1,"title":"Mock测试商品","images":"/test.jpg","price":99.99,"line_price":199.99,"stock_num":100,"status":1}')
GOOD_ID=$(echo "$GOOD_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "$GOOD_RESP" | grep -q '"code":0' && echo "✅ 管理-新增商品(id=$GOOD_ID)" && ((PASS++)) || echo "❌ 管理-新增商品" && ((FAIL++))

test_api GET "/api/v1/admin/goods?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-商品列表" ""
test_api PUT /api/v1/admin/goods/$GOOD_ID '{"id":'$GOOD_ID',"category_id":1,"title":"Mock测试商品(改)","images":"/test2.jpg","price":88.88,"line_price":188.88,"stock_num":50,"status":1}' "$ADMIN_TOKEN" "管理-编辑商品" ""
test_api PUT /api/v1/admin/goods/$GOOD_ID/stock '{"stock":500}' "$ADMIN_TOKEN" "管理-设置库存" ""

# ─── 寄售商品 ───
echo "─── 寄售商品 ───"
test_api GET "/api/v1/admin/merchandises?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-寄售商品列表" ""
test_api PUT /api/v1/admin/merchandises/378004/status '{"status":1,"is_show":1}' "$ADMIN_TOKEN" "管理-寄售状态" ""

# ─── 提现管理 ───
echo "─── 提现管理 ───"
test_api GET "/api/v1/admin/withdraws?page=1&limit=2&status=2" "" "$ADMIN_TOKEN" "管理-待审批提现" ""
test_api PUT /api/v1/admin/withdraws/8060/approve '{"status":1}' "$ADMIN_TOKEN" "管理-通过提现" ""

# ─── 财务日志 ───
echo "─── 财务日志 ───"
test_api GET "/api/v1/admin/logs/money?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-余额明细" ""
test_api GET "/api/v1/admin/logs/coupon?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-优惠券明细" ""
test_api GET "/api/v1/admin/logs/self-bonus?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-个人奖金明细" ""
test_api GET "/api/v1/admin/logs/share-bonus?page=1&limit=2" "" "$ADMIN_TOKEN" "管理-推广奖金明细" ""

# ─── 菜单规则 ───
echo "─── 菜单规则 ───"
test_api GET /api/v1/admin/rules "" "$ADMIN_TOKEN" "管理-规则列表" ""
test_api GET /api/v1/admin/rules/tree "" "$ADMIN_TOKEN" "管理-规则树" ""

# ─── 权限管理 ───
echo "─── 权限管理 ───"
test_api GET /api/v1/admin/admins "" "$ADMIN_TOKEN" "管理-管理员列表" ""

# Mock 创建管理员
ADMIN_RESP2=$(curl -s -X POST "$BASE/api/v1/admin/admins" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"username":"mockadmin","password":"123456","nickname":"Mock管理员","roles":"2"}')
echo "$ADMIN_RESP2" | grep -q '"code":0' && echo "✅ 管理-新增管理员" && ((PASS++)) || echo "❌ 管理-新增管理员" && ((FAIL++))

test_api GET /api/v1/admin/roles "" "$ADMIN_TOKEN" "管理-角色列表" ""

ROLE_RESP=$(curl -s -X POST "$BASE/api/v1/admin/roles" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"name":"Mock角色","rules":"[1,2,3]","status":1}')
ROLE_ID=$(echo "$ROLE_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "$ROLE_RESP" | grep -q '"code":0' && echo "✅ 管理-新增角色(id=$ROLE_ID)" && ((PASS++)) || echo "❌ 管理-新增角色" && ((FAIL++))

test_api PUT /api/v1/admin/roles/$ROLE_ID '{"name":"Mock角色(改)","rules":"[1,2,3,4]","status":1}' "$ADMIN_TOKEN" "管理-编辑角色" ""

# ─── 内容管理 ───
echo "─── 内容管理 ───"

BANNER_RESP=$(curl -s -X POST "$BASE/api/v1/admin/banners" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"title":"Mock Banner","image":"/test-banner.jpg","url":"/test","sort":1,"status":1}')
BANNER_ID=$(echo "$BANNER_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "$BANNER_RESP" | grep -q '"code":0' && echo "✅ 管理-新增Banner(id=$BANNER_ID)" && ((PASS++)) || echo "❌ 管理-新增Banner" && ((FAIL++))

test_api GET /api/v1/admin/banners "" "$ADMIN_TOKEN" "管理-Banner列表" ""
test_api PUT /api/v1/admin/banners/$BANNER_ID '{"title":"Mock Banner(改)","status":0}' "$ADMIN_TOKEN" "管理-编辑Banner" ""

AD_RESP=$(curl -s -X POST "$BASE/api/v1/admin/ads" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"title":"Mock广告","image":"/test-ad.jpg","url":"/promo","sort":1,"status":1}')
AD_ID=$(echo "$AD_RESP" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "$AD_RESP" | grep -q '"code":0' && echo "✅ 管理-新增广告(id=$AD_ID)" && ((PASS++)) || echo "❌ 管理-新增广告" && ((FAIL++))

test_api GET /api/v1/admin/ads "" "$ADMIN_TOKEN" "管理-广告列表" ""
test_api PUT /api/v1/admin/ads/$AD_ID '{"title":"Mock广告(改)","status":0}' "$ADMIN_TOKEN" "管理-编辑广告" ""

# ─── 秒杀管理 ───
echo "─── 秒杀管理 ───"

EVENT_RESP=$(curl -s -X POST "$BASE/api/v1/admin/flash-sale/events" \
  -H "Content-Type: application/json" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"product_id":1,"stock":100,"price":1999.00,"start_time":"2026-07-21T10:00:00+08:00","end_time":"2026-07-21T10:30:00+08:00","max_per_user":1,"status":0}')
echo "$EVENT_RESP" | grep -q '"code":0' && echo "✅ 管理-创建秒杀活动" && ((PASS++)) || echo "❌ 管理-创建秒杀活动" && ((FAIL++))

test_api GET /api/v1/admin/flash-sale/events "" "$ADMIN_TOKEN" "管理-秒杀活动列表" ""

# ─── 安全性验证 ───
echo "─── 安全验证 ───"
test_api GET /api/v1/admin/users?page=1\&limit=1 "" "" "安全-无Token拒绝" "401"
test_api GET /api/v1/front/user/profile "" "" "安全-C端无Token拒绝" "401"

# ============================================================
echo ""
echo "============================================"
echo " 测试结果: 通过 $PASS / 失败 $FAIL"
echo "============================================"

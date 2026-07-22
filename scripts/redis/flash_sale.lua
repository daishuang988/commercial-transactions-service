-- ============================================================
-- 秒杀核心 Lua 脚本（Redis 原子执行，防超卖）
--
-- KEYS[1] = product:stock:{product_id}         商品库存
-- KEYS[2] = order:record:{user_id}:{product_id} 防重复购买标记
-- KEYS[3] = order:stream                        订单Stream(异步落库)
--
-- ARGV[1] = stock         当前库存(用于初始判断)
-- ARGV[2] = user_id       用户ID
-- ARGV[3] = product_id    商品ID
-- ARGV[4] = price         成交价格
-- ARGV[5] = now           当前时间
--
-- 返回值: {code, message}
--   1, success      抢购成功
--   0, sold_out     已售罄
--   0, already_bought 已购买过
-- ============================================================

local stock_key   = KEYS[1]
local record_key  = KEYS[2]
local stream_key  = KEYS[3]
local user_id     = ARGV[2]
local product_id  = ARGV[3]
local price       = ARGV[4]
local now         = ARGV[5]

-- 1. 检查是否已购买（24小时内的重复请求）
local already = redis.call('GET', record_key)
if already then
    return {0, 'already_bought'}
end

-- 2. 检查库存并原子扣减
local current = tonumber(redis.call('GET', stock_key))
if current == nil or current <= 0 then
    return {0, 'sold_out'}
end
redis.call('DECR', stock_key)

-- 3. 标记已购买（24小时过期，同一天不重复）
redis.call('SETEX', record_key, 86400, '1')

-- 4. 写入Stream，由Worker异步消费落库到MySQL
local order = string.format(
    '{"user_id":"%s","product_id":"%s","price":"%s","time":"%s"}',
    user_id, product_id, price, now
)
redis.call('XADD', stream_key, '*', 'data', order)

return {1, 'success'}

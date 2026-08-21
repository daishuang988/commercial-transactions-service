#!/bin/bash
# 鑫启达老系统 8-21 冻结/隐藏 回滚脚本
# 用法: bash rollback_0821.sh
# 冻结回滚: status 恢复 before_snapshot_0821_user.txt 里的原值
# 隐藏回滚: is_show 恢复 before_snapshot_0821_merch.txt 里的原值
COOKIE='PHPSID=1a6be310cba1da41a06805f3cda605d0'
BASE='https://api.srdsmgs.com'

echo "== 回滚用户冻结 =="
while read id old_status; do
  code=$(curl -s --max-time 20 -X POST "$BASE/app/admin/user/update" -H "Cookie: $COOKIE" -d "id=$id&status=$old_status" | grep -o '"code":[0-9]*' | head -1)
  echo "$id -> status=$old_status $code"
done < before_snapshot_0821_user.txt

echo "== 回滚商品隐藏 =="
while read id old_show old_status; do
  code=$(curl -s --max-time 20 -X POST "$BASE/app/admin/merchandise/update" -H "Cookie: $COOKIE" -d "id=$id&is_show=$old_show" | grep -o '"code":[0-9]*' | head -1)
  echo "$id -> is_show=$old_show $code"
done < before_snapshot_0821_merch.txt

echo "== 回滚完成 =="

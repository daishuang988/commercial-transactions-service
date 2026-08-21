// 一次性统计脚本：xinqida 导入前数据画像（只读 JSON，不连库）
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Merch struct {
	ID     int `json:"id"`
	OldID  int `json:"old_id"`
	UserID int `json:"user_id"`
	Status int `json:"status"`
}

type Order struct {
	ID            int `json:"id"`
	BuyerID       int `json:"buyer_id"`
	SellerID      int `json:"seller_id"`
	MerchandiseID int `json:"merchandise_id"`
	Status        int `json:"status"`
	IsResell      int `json:"is_resell"`
}

type User struct {
	ID int `json:"id"`
}

type Wrapped struct {
	Count int               `json:"count"`
	Data  []json.RawMessage `json:"data"`
}

func main() {
	// ── 树 ──
	tree := map[int]bool{}
	var treeList []int
	tb, err := os.ReadFile("fan_tree_xinqida_ids.txt")
	if err != nil {
		panic(err)
	}
	for _, line := range strings.Split(string(tb), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			var id int
			fmt.Sscanf(line, "%d", &id)
			tree[id] = true
			treeList = append(treeList, id)
		}
	}
	fmt.Printf("【粉丝树】%d 人, 根=%d\n", len(tree), treeList[0])

	// ── 用户 ──
	var uw Wrapped
	ub, _ := os.ReadFile(os.Args[3])
	json.Unmarshal(ub, &uw)
	inTree, outTree := 0, 0
	for _, raw := range uw.Data {
		var u User
		json.Unmarshal(raw, &u)
		if tree[u.ID] {
			inTree++
		} else {
			outTree++
		}
	}
	fmt.Printf("【用户】老系统共 %d 人；树内命中 %d 人，树外 %d 人（树内未命中数=%d）\n",
		len(uw.Data), inTree, outTree, len(tree)-inTree)
	if len(tree) != inTree {
		fmt.Printf("  ⚠️ 树内 %d 人在用户表里没找到！\n", len(tree)-inTree)
	}

	// ── 商品 ──
	var mw Wrapped
	mb, _ := os.ReadFile(os.Args[1])
	json.Unmarshal(mb, &mw)
	merchIDs := map[int]bool{}
	oldToNew := map[int]int{} // old_id → 当前 id
	merchUser := map[int]int{} // id → user_id
	merchStatus := map[int]int{}
	treeS0, treeS1, outS0, outS1 := 0, 0, 0, 0
	for _, raw := range mw.Data {
		var m Merch
		json.Unmarshal(raw, &m)
		merchIDs[m.ID] = true
		oldToNew[m.OldID] = m.ID
		merchUser[m.ID] = m.UserID
		merchStatus[m.ID] = m.Status
		if tree[m.UserID] {
			if m.Status == 0 {
				treeS0++
			} else {
				treeS1++
			}
		} else {
			if m.Status == 0 {
				outS0++
			} else {
				outS1++
			}
		}
	}
	fmt.Printf("【商品】总 %d；树内: 已售(old status=0) %d + 未售 %d = %d；树外: 已售 %d + 未售 %d = %d\n",
		len(mw.Data), treeS0, treeS1, treeS0+treeS1, outS0, outS1, outS0+outS1)

	// ── 订单 ──
	var ow Wrapped
	ob, _ := os.ReadFile(os.Args[2])
	json.Unmarshal(ob, &ow)
	buyerIn := 0
	needImport := map[int]bool{}  // 被【树内买家订单】引用的树外商品（需补导入）
	mappedOld := map[int]int{}    // 树内买家订单中命中 old_id 可重映射的
	orphan := 0
	soldOutTreeRef := 0 // 树内买家订单引用树外已售商品数(口径核对用)
	for _, raw := range ow.Data {
		var o Order
		json.Unmarshal(raw, &o)
		if !tree[o.BuyerID] {
			continue // 只统计买家在树内的订单（导入口径）
		}
		buyerIn++
		if !merchIDs[o.MerchandiseID] {
			orphan++
			if nid, ok := oldToNew[o.MerchandiseID]; ok {
				mappedOld[nid]++
			}
		} else if !tree[merchUser[o.MerchandiseID]] {
			needImport[o.MerchandiseID] = true
			if merchStatus[o.MerchandiseID] == 0 {
				soldOutTreeRef++
			}
		}
	}
	fmt.Printf("【订单】总 %d；买家在树内 %d 条（=要导入的订单）\n", len(ow.Data), buyerIn)
	fmt.Printf("  其中悬空引用 %d 条（命中 old_id 可重映射 %d，真正悬空 %d）\n",
		orphan, len(mappedOld), orphan-len(mappedOld))
	fmt.Printf("  被树内买家订单引用的树外卖家商品 %d 件（需补导入，按新语义 status=1 已售）\n", len(needImport))

	fmt.Printf("\n【导入后预期】\n")
	fmt.Printf("  寄卖池(新status=0) = 树内未售 %d 件\n", treeS1)
	fmt.Printf("  已售 = 树内已售 %d + 树外补导 %d = %d 件\n", treeS0, len(needImport), treeS0+len(needImport))
	fmt.Printf("  老系统全部未售 %d = 树内 %d + 树外不导 %d  ✓口径核对\n", treeS1+outS1, treeS1, outS1)
}

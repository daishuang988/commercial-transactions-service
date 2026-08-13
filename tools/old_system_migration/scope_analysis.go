package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type orderRow struct {
	ID       int64  `json:"id"`
	SellerID int64  `json:"seller_id"`
	BuyerID  int64  `json:"buyer_id"`
	Status   int    `json:"status"`
	IsResell int    `json:"is_resell"`
	MerchID  int64  `json:"merchandise_id"`
}

func main() {
	// 读取粉丝树 ID 列表
	idsRaw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	tree := map[int64]bool{}
	for _, line := range strings.Split(string(idsRaw), "\n") {
		var id int64
		fmt.Sscanf(strings.TrimSpace(line), "%d", &id)
		if id > 0 {
			tree[id] = true
		}
	}
	fmt.Printf("粉丝树人数: %d\n\n", len(tree))

	// 分析订单
	ordersRaw, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	var orders struct {
		Data []orderRow `json:"data"`
	}
	json.Unmarshal(ordersRaw, &orders)

	buyerCnt, sellerCnt, bothCnt, outBoth := 0, 0, 0, 0
	resell0, resell1 := 0, 0
	statusDist := map[int]int{}
	linkedMerch := map[int64]bool{}
	for _, o := range orders.Data {
		inBuyer := tree[o.BuyerID]
		inSeller := tree[o.SellerID]
		if inBuyer && inSeller {
			bothCnt++
		} else if inBuyer {
			buyerCnt++
		} else if inSeller {
			sellerCnt++
		} else {
			outBoth++
			continue
		}
		if o.IsResell == 1 {
			resell1++
		} else {
			resell0++
		}
		statusDist[o.Status]++
		linkedMerch[o.MerchID] = true
	}
	fmt.Printf("订单分析 (总数 %d):\n", len(orders.Data))
	fmt.Printf("  双方都在树内: %d\n", bothCnt)
	fmt.Printf("  买家在树内(卖家树外): %d\n", buyerCnt)
	fmt.Printf("  卖家在树内(买家树外): %d\n", sellerCnt)
	fmt.Printf("  与树无关: %d\n", outBoth)
	fmt.Printf("\n  其中 is_resell=0: %d, is_resell=1: %d\n", resell0, resell1)
	fmt.Printf("  状态分布: %v\n", statusDist)
	fmt.Printf("  关联寄售商品数: %d\n", len(linkedMerch))

	// 分析寄售商品
	merchRaw, err := os.ReadFile(os.Args[3])
	if err != nil {
		panic(err)
	}
	var merchs struct {
		Data []struct {
			ID     int64 `json:"id"`
			UserID int64 `json:"user_id"`
			Status int   `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(merchRaw, &merchs)
	treeMerch := 0
	for _, m := range merchs.Data {
		if tree[m.UserID] {
			treeMerch++
		}
	}
	fmt.Printf("\n寄售商品分析 (总数 %d):\n", len(merchs.Data))
	fmt.Printf("  树内用户的寄售商品: %d\n", treeMerch)
}

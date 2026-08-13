package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type userData struct {
	Data []struct {
		ID       int64 `json:"id"`
		PID      int64 `json:"pid"`
		Username string `json:"username"`
		Nickname string `json:"nickname"`
		Mobile   string `json:"mobile"`
	} `json:"data"`
}

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var rootID int64
	fmt.Sscanf(os.Args[2], "%d", &rootID)

	var d userData
	if err := json.Unmarshal(raw, &d); err != nil {
		panic(err)
	}

	// pid → 直接粉丝
	children := map[int64][]int64{}
	users := map[int64]string{}
	for _, u := range d.Data {
		users[u.ID] = u.Nickname + "(" + u.Username + ")"
		if u.PID > 0 {
			children[u.PID] = append(children[u.PID], u.ID)
		}
	}

	// BFS 遍历整棵粉丝树
	visited := map[int64]bool{}
	levels := [][]int64{}
	queue := []int64{rootID}
	visited[rootID] = true
	for len(queue) > 0 {
		var next []int64
		for _, id := range queue {
			next = append(next, children[id]...)
		}
		level := []int64{}
		for _, id := range next {
			if !visited[id] {
				visited[id] = true
				level = append(level, id)
			}
		}
		if len(level) == 0 {
			break
		}
		sort.Slice(level, func(i, j int) bool { return level[i] < level[j] })
		levels = append(levels, level)
		queue = level
	}

	total := len(visited)
	fmt.Printf("根用户: %d %s\n", rootID, users[rootID])
	fmt.Printf("粉丝树总人数: %d\n\n", total)
	for i, lv := range levels {
		fmt.Printf("── 第 %d 层 (%d人): ", i+1, len(lv))
		for j, id := range lv {
			if j > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%d", id)
		}
		fmt.Println()
	}

	// 输出 ID 列表（方便后续 SQL 用）
	ids := make([]int64, 0, total)
	for id := range visited {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	fmt.Print("\nID列表: ")
	for i, id := range ids {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%d", id)
	}
	fmt.Println()
}

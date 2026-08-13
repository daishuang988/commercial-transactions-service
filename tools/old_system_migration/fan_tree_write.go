package main

// fan_tree_write.go — 重算粉丝树并写出 ID 列表文件
// 用法: fan_tree_write.exe <user_FULL.json> <rootID> <outIDsFile>

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	raw, _ := os.ReadFile(os.Args[1])
	var rootID int64
	fmt.Sscanf(os.Args[2], "%d", &rootID)

	var d struct {
		Data []struct {
			ID  int64 `json:"id"`
			PID int64 `json:"pid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		panic(err)
	}
	children := map[int64][]int64{}
	for _, u := range d.Data {
		if u.PID > 0 {
			children[u.PID] = append(children[u.PID], u.ID)
		}
	}
	visited := map[int64]bool{rootID: true}
	levels := [][]int64{}
	queue := []int64{rootID}
	for len(queue) > 0 {
		var next []int64
		for _, id := range queue {
			next = append(next, children[id]...)
		}
		var level []int64
		for _, id := range next {
			if !visited[id] {
				visited[id] = true
				level = append(level, id)
			}
		}
		if len(level) == 0 {
			break
		}
		levels = append(levels, level)
		queue = level
	}
	ids := make([]int64, 0, len(visited))
	for id := range visited {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var sb strings.Builder
	for _, id := range ids {
		sb.WriteString(strconv.FormatInt(id, 10) + "\n")
	}
	if err := os.WriteFile(os.Args[3], []byte(sb.String()), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("粉丝树总人数: %d\n", len(visited))
	for i, lv := range levels {
		fmt.Printf("  第 %d 层: %d 人\n", i+1, len(lv))
	}
	fmt.Printf("ID列表已写入: %s\n", os.Args[3])
}

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryStore 内存存储（Redis 的轻量替代，单机场景秒杀足够）
// 生产环境多机部署时替换为 Redis 即可
type MemoryStore struct {
	mu       sync.RWMutex
	stock    map[int64]*int64          // product_id → 剩余库存
	records  map[string]int64          // "user_id:product_id" → 过期时间戳
	orderCh  chan OrderMessage         // 订单消息通道
}

// OrderMessage 订单消息
type OrderMessage struct {
	UserID    int64  `json:"user_id"`
	ProductID int64  `json:"product_id"`
	Price     string `json:"price"`
	Time      string `json:"time"`
}

var MemStore *MemoryStore

func InitMemoryStore() {
	MemStore = &MemoryStore{
		stock:   make(map[int64]*int64),
		records: make(map[string]int64),
		orderCh: make(chan OrderMessage, 100000), // 10万缓冲
	}
	go MemStore.cleanExpired()
}

// InitStock 初始化商品库存
func (m *MemoryStore) InitStock(productID int64, stock int) {
	v := int64(stock)
	m.mu.Lock()
	m.stock[productID] = &v
	m.mu.Unlock()
}

// GetStock 获取库存
func (m *MemoryStore) GetStock(productID int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.stock[productID]; ok {
		return int(atomic.LoadInt64(v))
	}
	return -1
}

// DeductStock 原子扣库存，返回 (success, reason)
func (m *MemoryStore) DeductStock(userID, productID int64, price string) (bool, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 检查库存
	v, ok := m.stock[productID]
	if !ok || atomic.LoadInt64(v) <= 0 {
		return false, "sold_out"
	}

	// 2. 扣库存
	atomic.AddInt64(v, -1)

	// 3. 发送订单消息
	m.orderCh <- OrderMessage{
		UserID:    userID,
		ProductID: productID,
		Price:     price,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	}

	return true, "success"
}

// OrderChan 获取订单消息通道
func (m *MemoryStore) OrderChan() <-chan OrderMessage {
	return m.orderCh
}

// cleanExpired 定期清理过期记录
func (m *MemoryStore) cleanExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		now := time.Now().Unix()
		m.mu.Lock()
		for k, exp := range m.records {
			if exp < now {
				delete(m.records, k)
			}
		}
		m.mu.Unlock()
	}
}

// ─── 兼容 Redis 接口的库存查询 ───

func GetProductStock(ctx context.Context, productID int64) (int, error) {
	if RDB != nil {
		key := fmt.Sprintf("product:stock:%d", productID)
		val, err := RDB.Get(ctx, key).Int()
		if err == nil {
			return val, nil
		}
	}
	if MemStore != nil {
		return MemStore.GetStock(productID), nil
	}
	return 0, fmt.Errorf("no store available")
}

func InitProductStock(ctx context.Context, productID int64, stock int) error {
	if RDB != nil {
		key := fmt.Sprintf("product:stock:%d", productID)
		return RDB.Set(ctx, key, stock, 0).Err()
	}
	if MemStore != nil {
		MemStore.InitStock(productID, stock)
		return nil
	}
	return fmt.Errorf("no store available")
}

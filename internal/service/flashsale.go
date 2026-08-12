package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
)

// FlashSaleResult 抢购结果
type FlashSaleResult struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	OrderID int64  `json:"order_id,omitempty"`
}

// ExecuteFlashSale 执行秒杀（Redis > MemoryStore > 不可用）
// 注: 时间/权限校验已在 handler 层完成，此处只做执行
func ExecuteFlashSale(ctx context.Context, userID, productID int64, price float64) (*FlashSaleResult, error) {
	// 优先走 Redis Lua 脚本，否则走内存原子操作
	if repository.RDB != nil {
		return executeRedis(ctx, userID, productID, price)
	}
	if repository.MemStore != nil {
		return executeMemory(userID, productID, price)
	}
	return &FlashSaleResult{Success: false, Msg: "秒杀服务未就绪"}, nil
}

func executeRedis(ctx context.Context, userID, productID int64, price float64) (*FlashSaleResult, error) {
	stockKey := fmt.Sprintf("product:stock:%d", productID)
	streamKey := "order:stream"
	now := time.Now().In(repository.CSTLocation()).Format("2006-01-02 15:04:05")

	result, err := repository.RDB.Eval(ctx, repository.FlashSaleLuaScript,
		[]string{stockKey, streamKey},
		userID, productID, fmt.Sprintf("%.2f", price), now).Result()
	if err != nil {
		return nil, fmt.Errorf("Redis执行失败: %w", err)
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) < 2 {
		return &FlashSaleResult{Success: false, Msg: "系统错误"}, nil
	}
	code, _ := strconv.Atoi(fmt.Sprintf("%v", arr[0]))
	msg := fmt.Sprintf("%v", arr[1])
	return parseResult(code, msg), nil
}

func executeMemory(userID, productID int64, price float64) (*FlashSaleResult, error) {
	ok, reason := repository.MemStore.DeductStock(userID, productID, fmt.Sprintf("%.2f", price))
	if ok {
		return &FlashSaleResult{Success: true, Msg: "抢购成功"}, nil
	}
	return &FlashSaleResult{Success: false, Msg: reasonToMsg(reason)}, nil
}

func parseResult(code int, msg string) *FlashSaleResult {
	switch msg {
	case "sold_out":
		return &FlashSaleResult{Success: false, Msg: "已售罄"}
	case "success":
		return &FlashSaleResult{Success: true, Msg: "抢购成功"}
	default:
		return &FlashSaleResult{Success: code == 1, Msg: msg}
	}
}

func reasonToMsg(reason string) string {
	switch reason {
	case "sold_out":
		return "已售罄"
	default:
		return reason
	}
}

// StreamOrderData Redis Stream中的订单数据
type StreamOrderData struct {
	UserID    string `json:"user_id"`
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
	Time      string `json:"time"`
}

// StartOrderWorker 启动异步落库Worker
func StartOrderWorker(ctx context.Context, workerCount int, batchSize int, intervalMs int) {
	if repository.RDB != nil {
		for i := 0; i < workerCount; i++ {
			go orderWorker(ctx, i, batchSize, intervalMs)
		}
		log.Printf("启动 %d 个 Redis 订单落库Worker", workerCount)
	} else if repository.MemStore != nil {
		for i := 0; i < workerCount; i++ {
			go memoryWorker(ctx, i, batchSize, intervalMs)
		}
		log.Printf("启动 %d 个内存订单落库Worker", workerCount)
	}
}

// StartEventStatusUpdater 定时更新秒杀活动状态
func StartEventStatusUpdater(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				now := time.Now().In(repository.CSTLocation())
				// 进行中→已结束
				repository.DB.Exec(
					"UPDATE flash_sale_events SET status=2 WHERE status=1 AND end_time <= ?", now)
				// 未开始→进行中
				repository.DB.Exec(
					"UPDATE flash_sale_events SET status=1 WHERE status=0 AND start_time <= ?", now)
			}
		}
	}()
}

// StartDailyCouponSettlement 凌晨结算优惠券（抢单奖励）
func StartDailyCouponSettlement(ctx context.Context) {
	go func() {
		lastRun := ""
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				today := time.Now().In(repository.CSTLocation()).Format("20060102")
				if today == lastRun {
					continue
				}
				hour, min := time.Now().In(repository.CSTLocation()).Hour(), time.Now().In(repository.CSTLocation()).Minute()
				if hour != 23 || min < 59 {
					continue
				}
				lastRun = today
				log.Printf("开始凌晨优惠券结算...")
				SettleDailyCoupons()
				log.Printf("凌晨优惠券结算完成")
			}
		}
	}()
}

func SettleDailyCoupons() {
	orderRewardRate := parseConfigRate("order_reward_rate")
	if orderRewardRate <= 0 { return }

	var orders []model.Order
	repository.DB.Where("status = 2 AND coupon_settled = 0").Find(&orders)
	for _, o := range orders {
		coupon := o.TotalMoney * orderRewardRate
		if coupon <= 0 { continue }
		before := getWalletBal(o.BuyerID, "coupon")
		after := before + coupon
		repository.DB.Exec("INSERT INTO coupon_logs (user_id,type,money,`before`,`after`,memo,created_at,updated_at) VALUES(?,1,?,?,?,?,NOW(),NOW())",
			o.BuyerID, coupon, before, after, "今日收益")
		repository.DB.Exec("UPDATE user_wallets SET coupon = ?, updated_at = NOW() WHERE user_id = ?", after, o.BuyerID)
		repository.DB.Model(&o).Update("coupon_settled", int8(1))
	}

	// 快照：今日卖数据 → yesterday_sell_count，然后归零今日计数
	repository.DB.Exec(`
		UPDATE users u
		SET yesterday_sell_count = (
			SELECT COUNT(*) FROM orders o
			WHERE o.seller_id = u.id AND o.status = 2 AND DATE(o.confirm_time) = CURDATE()
		),
		today_buy_total = 0,
		today_buy_count = 0,
		today_sell_total = 0,
		updated_at = NOW()
	`)
	log.Printf("用户昨日卖快照 + 今日计数归零完成")
}

func getWalletBal(userID int64, field string) float64 {
	var v float64
	repository.DB.Raw("SELECT COALESCE("+field+",0) FROM user_wallets WHERE user_id = ?", userID).Scan(&v)
	return v
}

func parseConfigRate(key string) float64 {
	s := repository.GetConfigStr(key)
	if s == "" { return 0 }
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func memoryWorker(ctx context.Context, id int, batchSize int, intervalMs int) {
	ch := repository.MemStore.OrderChan()
	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	var batch []model.FlashSaleRecord
	for {
		select {
		case <-ctx.Done():
			flushBatch(&batch)
			return
		case msg := <-ch:
			price, _ := strconv.ParseFloat(msg.Price, 64)

			now := time.Now().In(repository.CSTLocation())
			batch = append(batch, model.FlashSaleRecord{
				UserID: msg.UserID, ProductID: msg.ProductID,
				Price: price, Status: 1, CreatedAt: now,
			})

			if len(batch) >= batchSize {
				flushBatch(&batch)
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushBatch(&batch)
			}
		}
	}
}

func flushBatch(batch *[]model.FlashSaleRecord) {
	if len(*batch) == 0 {
		return
	}
	if err := repository.BatchCreateFlashSaleRecords(*batch); err != nil {
		log.Printf("Worker: 批量写入失败: %v", err)
		return
	}
	log.Printf("Worker: 写入 %d 条订单", len(*batch))
	*batch = (*batch)[:0]
}

func orderWorker(ctx context.Context, id int, batchSize int, intervalMs int) {
	streamKey := "order:stream"
	groupName := "order-consumers"
	consumerName := fmt.Sprintf("worker-%d", id)

	// 创建消费者组（忽略已存在的错误）
	repository.RDB.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			streams, err := repository.RDB.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    groupName,
				Consumer: consumerName,
				Streams:  []string{streamKey, ">"},
				Count:    int64(batchSize),
				Block:    time.Duration(intervalMs) * time.Millisecond,
			}).Result()

			if err != nil || len(streams) == 0 || len(streams[0].Messages) == 0 {
				continue
			}

			var records []model.FlashSaleRecord
			var msgIDs []string
			for _, msg := range streams[0].Messages {
				msgIDs = append(msgIDs, msg.ID)
				dataStr := msg.Values["data"].(string)
				var data StreamOrderData
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					continue
				}
				uid, _ := strconv.ParseInt(data.UserID, 10, 64)
				pid, _ := strconv.ParseInt(data.ProductID, 10, 64)
				price, _ := strconv.ParseFloat(data.Price, 64)
				t, _ := time.Parse("2006-01-02 15:04:05", data.Time)

				records = append(records, model.FlashSaleRecord{
					UserID:    uid,
					ProductID: pid,
					Price:     price,
					Status:    1,
					CreatedAt: t,
				})
			}

			if len(records) > 0 {
				if err := repository.BatchCreateFlashSaleRecords(records); err != nil {
					log.Printf("Worker-%d: 批量写入失败: %v", id, err)
					continue
				}
				// 确认消息
				for _, msgID := range msgIDs {
					repository.RDB.XAck(ctx, streamKey, groupName, msgID)
				}
				log.Printf("Worker-%d: 写入 %d 条订单", id, len(records))
			}
		}
	}
}

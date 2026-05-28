package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

// 异步聚合写 last_accessed_at，规避热点行频繁 UPDATE。
//
// 写入路径：把 (fileID, now) 写入一个固定 Redis ZSet，由后台 cron 周期 Flush。
// 选择 Redis ZSet 的原因：
//   - 单 key 即可承载所有待 flush 文件；
//   - ZADD GT 选项保证同一文件只保留“最新一次”访问时间，去抖到秒级粒度；
//   - 取数后按时间戳分组成少量 UPDATE，消除热点行高频写。
//
// 兜底：Redis 不可用 / ZADD 出错时退化为直接同步 UpdateById，确保不丢访问时间。
const (
	fileAccessPendingKey  = "file_resource:last_accessed_pending"
	fileAccessFlushBatch  = 1000
	fileAccessRedisTimeoutOp = 500 * time.Millisecond
)

// fileAccessStore 抽象出 ZSet 操作，便于单测。
type fileAccessStore interface {
	ZAddGT(ctx context.Context, key string, score float64, member string) error
	ZPopMin(ctx context.Context, key string, count int64) ([]redis.Z, error)
	// ZAddBatch 按原 score 回写一批成员，用于 Flush 阶段 UPDATE 失败时无损回滚。
	// 实现需保证：若同名成员已被新一次访问以更新的 score 写入，则保留较大者（不覆盖更新的 score）。
	ZAddBatch(ctx context.Context, key string, members []redis.Z) error
}

type redisFileAccessStore struct {
	client redis.Cmdable
}

func (s redisFileAccessStore) ZAddGT(ctx context.Context, key string, score float64, member string) error {
	if s.client == nil {
		return errors.New("redis client unavailable")
	}
	return s.client.ZAddArgs(ctx, key, redis.ZAddArgs{
		GT: true,
		Members: []redis.Z{
			{Score: score, Member: member},
		},
	}).Err()
}

func (s redisFileAccessStore) ZPopMin(ctx context.Context, key string, count int64) ([]redis.Z, error) {
	if s.client == nil {
		return nil, errors.New("redis client unavailable")
	}
	return s.client.ZPopMin(ctx, key, count).Result()
}

// ZAddBatch 使用 pipeline 一次提交所有回写命令，并加 GT 选项避免覆盖新一轮访问写入。
func (s redisFileAccessStore) ZAddBatch(ctx context.Context, key string, members []redis.Z) error {
	if s.client == nil {
		return errors.New("redis client unavailable")
	}
	if len(members) == 0 {
		return nil
	}
	pipe := s.client.Pipeline()
	for _, m := range members {
		pipe.ZAddArgs(ctx, key, redis.ZAddArgs{
			GT:      true,
			Members: []redis.Z{m},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

// FileAccessAggregator 负责把高频 last_accessed_at 写入合并到 Redis，再周期落库。
type FileAccessAggregator struct {
	store     fileAccessStore
	dbFn      func() (*gorm.DB, error)
	tableName string
	now       func() time.Time
	batchSize int64
}

var (
	defaultFileAccessAggregator     *FileAccessAggregator
	defaultFileAccessAggregatorOnce sync.Once
)

// DefaultFileAccessAggregator 返回全局聚合器，使用项目默认的 Redis / DB。
func DefaultFileAccessAggregator() *FileAccessAggregator {
	defaultFileAccessAggregatorOnce.Do(func() {
		defaultFileAccessAggregator = &FileAccessAggregator{
			store:     redisFileAccessStore{client: nil}, // 延迟取，避免初始化期 Redis 未就绪
			dbFn:      func() (*gorm.DB, error) { return model.GetDB() },
			tableName: model.NewUploadFiles().TableName(),
			now:       time.Now,
			batchSize: fileAccessFlushBatch,
		}
	})
	return defaultFileAccessAggregator
}

// SetFileAccessAggregatorForTesting 仅供测试使用：替换默认聚合器并返回还原函数。
func SetFileAccessAggregatorForTesting(agg *FileAccessAggregator) func() {
	defaultFileAccessAggregatorOnce.Do(func() {})
	previous := defaultFileAccessAggregator
	defaultFileAccessAggregator = agg
	return func() {
		defaultFileAccessAggregator = previous
	}
}

// Mark 记录一次文件访问。Redis 不可用时退化为同步 UpdateById（best-effort，错误吞掉）。
//
// 注意：写入路径必须不阻塞且不返回错误，保留调用方 `_ = ...` 的忽略错误语义。
func (a *FileAccessAggregator) Mark(uploadFile *model.UploadFiles) {
	if a == nil || uploadFile == nil || uploadFile.ID == 0 {
		return
	}
	now := a.now()

	store := a.resolveStore()
	if store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), fileAccessRedisTimeoutOp)
		defer cancel()
		if err := store.ZAddGT(ctx, fileAccessPendingKey, float64(now.Unix()), strconv.FormatUint(uint64(uploadFile.ID), 10)); err == nil {
			return
		} else {
			// Redis 异常仅记录 debug 级别，避免高频访问下日志风暴。
			if log.Logger != nil {
				log.Logger.Debug("file access aggregator zadd failed, fallback to sync update",
					zap.Uint("file_id", uploadFile.ID),
					zap.Error(err),
				)
			}
		}
	}

	// 兜底：直接同步 UPDATE，确保不丢访问时间。
	_ = uploadFile.UpdateById(uploadFile.ID, map[string]any{"last_accessed_at": now})
}

// Flush 从 Redis 取出待写入项并按时间戳分组批量 UPDATE。返回实际写入条数与可能的错误。
//
// 设计要点：
//   - 使用 ZPOPMIN 消费式取出：成功取出即从 ZSet 移除，避免重复刷写；
//   - UPDATE 失败时按原 score pipeline 回写到 ZSet（best-effort），保证数据不丢；
//   - 回写自身失败时仅记录 Error 日志，不再无限重试（同一 batch 内只重试一次，
//     下次 cron 调度自然会再次尝试拉取真正残留的 ID）。
func (a *FileAccessAggregator) Flush(ctx context.Context) (int, error) {
	if a == nil {
		return 0, nil
	}
	store := a.resolveStore()
	if store == nil {
		return 0, nil
	}

	batchSize := a.batchSize
	if batchSize <= 0 {
		batchSize = fileAccessFlushBatch
	}

	popCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	items, err := store.ZPopMin(popCtx, fileAccessPendingKey, batchSize)
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	// 按时间戳分组（同一秒内的多个 ID 共用一条 UPDATE）。
	// 同时记录每个 (id -> score) 映射，便于失败后准确回写原始 score（避免向前/向后偏移）。
	groups := make(map[int64][]uint64, 4)
	idScore := make(map[uint64]float64, len(items))
	for _, item := range items {
		member, ok := item.Member.(string)
		if !ok || member == "" {
			continue
		}
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil || id == 0 {
			continue
		}
		ts := int64(item.Score)
		groups[ts] = append(groups[ts], id)
		// 同一 ID 在 ZPOPMIN 结果里不会出现多次（ZSet 同名成员只有一条记录），无需去重。
		idScore[id] = item.Score
	}
	if len(groups) == 0 {
		return 0, nil
	}

	db, err := a.dbFn()
	if err != nil {
		// DB 不可用：把本批 ID 整体按原 score 回写，避免数据丢失。
		a.requeueFailedGroups(ctx, store, groups, idScore, nil)
		return 0, err
	}

	tableName := a.tableName
	if tableName == "" {
		tableName = model.NewUploadFiles().TableName()
	}

	// retried 用作本次 batch 内"已计入回写"的短路集合：单条 ID 在本次 Flush 内不会被重复入队。
	retried := make(map[uint64]struct{}, len(idScore))
	written := 0
	for ts, ids := range groups {
		// 使用 FROM_UNIXTIME 在 MySQL 上把时间戳转回 datetime；sqlite 测试时不会执行真实路径，
		// 测试聚合器写库流程时改走 GORM Updates，更便于 sqlite 模拟。
		if err := db.Table(tableName).
			Where("id IN ?", ids).
			Updates(map[string]any{"last_accessed_at": time.Unix(ts, 0)}).Error; err != nil {
			if log.Logger != nil {
				log.Logger.Warn("file access aggregator flush update failed, requeue pending ids",
					zap.Int64("ts", ts),
					zap.Int("ids", len(ids)),
					zap.Error(err))
			}
			a.requeueFailedGroups(ctx, store, map[int64][]uint64{ts: ids}, idScore, retried)
			continue
		}
		written += len(ids)
	}

	return written, nil
}

// requeueFailedGroups 把指定分组按原 score 回写到 pending ZSet。
// retried 为空时表示首次/整体回写，否则在本次 batch 内对每条 ID 仅回写一次，避免无限循环。
func (a *FileAccessAggregator) requeueFailedGroups(
	ctx context.Context,
	store fileAccessStore,
	groups map[int64][]uint64,
	idScore map[uint64]float64,
	retried map[uint64]struct{},
) {
	if store == nil || len(groups) == 0 {
		return
	}
	members := make([]redis.Z, 0, len(idScore))
	for _, ids := range groups {
		for _, id := range ids {
			if retried != nil {
				if _, ok := retried[id]; ok {
					continue
				}
				retried[id] = struct{}{}
			}
			score, ok := idScore[id]
			if !ok {
				continue
			}
			members = append(members, redis.Z{
				Score:  score,
				Member: strconv.FormatUint(id, 10),
			})
		}
	}
	if len(members) == 0 {
		return
	}

	requeueCtx, cancel := context.WithTimeout(ctx, fileAccessRedisTimeoutOp)
	defer cancel()
	if err := store.ZAddBatch(requeueCtx, fileAccessPendingKey, members); err != nil {
		if log.Logger != nil {
			log.Logger.Error("file access aggregator requeue pending ids failed",
				zap.Int("count", len(members)),
				zap.Error(err))
		}
	}
}

func (a *FileAccessAggregator) resolveStore() fileAccessStore {
	if a == nil {
		return nil
	}
	if a.store != nil {
		// 默认 redisFileAccessStore.client 为 nil 时，每次实时从 data.RedisClient() 取，保证 Redis 后期初始化也能拿到。
		if rs, ok := a.store.(redisFileAccessStore); ok && rs.client == nil {
			client := data.RedisClient()
			if client == nil {
				return nil
			}
			return redisFileAccessStore{client: client}
		}
		return a.store
	}
	client := data.RedisClient()
	if client == nil {
		return nil
	}
	return redisFileAccessStore{client: client}
}

// markFileAccessed 是 common.go 写入路径的入口，包装默认聚合器，保持调用点最小化。
func markFileAccessed(uploadFile *model.UploadFiles) {
	DefaultFileAccessAggregator().Mark(uploadFile)
}

// FlushFileAccessPending 是 cron handler 的入口。
func FlushFileAccessPending(ctx context.Context) error {
	written, err := DefaultFileAccessAggregator().Flush(ctx)
	if err != nil {
		if log.Logger != nil {
			log.Logger.Warn("flush file access pending failed", zap.Error(err))
		}
		return err
	}
	if written > 0 && log.Logger != nil {
		log.Logger.Debug("flush file access pending", zap.Int("written", written))
	}
	return nil
}

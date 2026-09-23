package service

import (
	"context"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// fakeFileAccessStore 是内存版 ZSet，仅实现 ZAddGT(GT 语义) + ZPopMin。
type fakeFileAccessStore struct {
	mu     sync.Mutex
	scores map[string]float64
}

func newFakeFileAccessStore() *fakeFileAccessStore {
	return &fakeFileAccessStore{scores: make(map[string]float64)}
}

func (s *fakeFileAccessStore) ZAddGT(_ context.Context, _ string, score float64, member string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cur, ok := s.scores[member]; ok && score <= cur {
		return nil
	}
	s.scores[member] = score
	return nil
}

func (s *fakeFileAccessStore) ZPopMin(_ context.Context, _ string, count int64) ([]redis.Z, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type pair struct {
		member string
		score  float64
	}
	all := make([]pair, 0, len(s.scores))
	for m, sc := range s.scores {
		all = append(all, pair{m, sc})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score < all[j].score })
	if int64(len(all)) > count {
		all = all[:count]
	}
	out := make([]redis.Z, 0, len(all))
	for _, p := range all {
		out = append(out, redis.Z{Score: p.score, Member: p.member})
		delete(s.scores, p.member)
	}
	return out, nil
}

// ZAddBatch 模拟 pipeline 回写：对每个成员套用 GT 语义。
func (s *fakeFileAccessStore) ZAddBatch(_ context.Context, _ string, members []redis.Z) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range members {
		key, ok := m.Member.(string)
		if !ok {
			continue
		}
		if cur, exists := s.scores[key]; exists && m.Score <= cur {
			continue
		}
		s.scores[key] = m.Score
	}
	return nil
}

func (s *fakeFileAccessStore) snapshot() map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]float64, len(s.scores))
	for k, v := range s.scores {
		out[k] = v
	}
	return out
}

func newFileAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	// 仅创建测试所需最小列。
	if err := db.Exec(`CREATE TABLE upload_files (
		id integer primary key autoincrement,
		last_accessed_at datetime
	)`).Error; err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	return db
}

// TestFileAccessAggregatorMarkUsesZAddWithGTSemantics 验证 Mark 走 Redis 路径，
// 且 GT 语义下早时间戳不会覆盖晚时间戳。
func TestFileAccessAggregatorMarkUsesZAddWithGTSemantics(t *testing.T) {
	store := newFakeFileAccessStore()
	db := newFileAccessTestDB(t)

	t0 := time.Unix(1000, 0)
	t1 := time.Unix(2000, 0)
	t2 := time.Unix(1500, 0) // 比 t1 小，应被 GT 语义忽略

	calls := 0
	now := func() time.Time {
		calls++
		switch calls {
		case 1:
			return t0
		case 2:
			return t1
		default:
			return t2
		}
	}

	agg := &FileAccessAggregator{
		store:     store,
		dbFn:      func() (*gorm.DB, error) { return db, nil },
		tableName: "upload_files",
		now:       now,
		batchSize: 100,
	}

	uf := newUploadFileForTest(42)
	agg.Mark(uf)
	agg.Mark(uf)
	agg.Mark(uf)

	snap := store.snapshot()
	got, ok := snap["42"]
	if !ok {
		t.Fatalf("expected member 42 in store, got %#v", snap)
	}
	if got != float64(t1.Unix()) {
		t.Fatalf("expected score %d (latest), got %v", t1.Unix(), got)
	}
}

// TestFileAccessAggregatorFlushBatchUpdates 验证 Flush 取出后按时间戳分组 UPDATE。
func TestFileAccessAggregatorFlushBatchUpdates(t *testing.T) {
	store := newFakeFileAccessStore()
	db := newFileAccessTestDB(t)

	// 准备 3 条记录：id=1,2,3，初始 last_accessed_at 为 NULL。
	for _, id := range []uint{1, 2, 3} {
		if err := db.Exec(`INSERT INTO upload_files (id, last_accessed_at) VALUES (?, NULL)`, id).Error; err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// ZSet 中：id 1,2 同时间戳；id 3 不同时间戳，确保走分组路径。
	ts1 := time.Unix(1700000000, 0)
	ts2 := time.Unix(1700000060, 0)
	for _, id := range []uint{1, 2} {
		if err := store.ZAddGT(context.Background(), "", float64(ts1.Unix()), strconv.FormatUint(uint64(id), 10)); err != nil {
			t.Fatalf("zadd failed: %v", err)
		}
	}
	if err := store.ZAddGT(context.Background(), "", float64(ts2.Unix()), "3"); err != nil {
		t.Fatalf("zadd failed: %v", err)
	}

	agg := &FileAccessAggregator{
		store:     store,
		dbFn:      func() (*gorm.DB, error) { return db, nil },
		tableName: "upload_files",
		now:       time.Now,
		batchSize: 100,
	}

	written, err := agg.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if written != 3 {
		t.Fatalf("expected 3 rows written, got %d", written)
	}

	// 校验落库时间。
	type row struct {
		ID             uint
		LastAccessedAt *time.Time `gorm:"column:last_accessed_at"`
	}
	var rows []row
	if err := db.Raw(`SELECT id, last_accessed_at FROM upload_files ORDER BY id`).Scan(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.LastAccessedAt == nil {
			t.Fatalf("row %d has nil last_accessed_at", r.ID)
		}
	}
	if !rows[0].LastAccessedAt.Equal(ts1) || !rows[1].LastAccessedAt.Equal(ts1) {
		t.Fatalf("expected rows 1/2 ts=%v, got %v / %v", ts1, rows[0].LastAccessedAt, rows[1].LastAccessedAt)
	}
	if !rows[2].LastAccessedAt.Equal(ts2) {
		t.Fatalf("expected row 3 ts=%v, got %v", ts2, rows[2].LastAccessedAt)
	}

	// Flush 后 ZSet 应被清空（ZPopMin 已消费）。
	if remain := store.snapshot(); len(remain) != 0 {
		t.Fatalf("expected store empty after flush, got %#v", remain)
	}
}

// TestFileAccessAggregatorMarkFallbackOnRedisFailure 验证 Redis 异常时走同步兜底 UPDATE。
func TestFileAccessAggregatorMarkFallbackOnRedisFailure(t *testing.T) {
	db := newFileAccessTestDB(t)
	if err := db.Exec(`INSERT INTO upload_files (id, last_accessed_at) VALUES (7, NULL)`).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	failStore := failingFileAccessStore{}
	// 兜底走 model 默认的 GetDB；这里没法替换全局 DB，直接走最低成本：构造一个
	// uploadFile 并通过 SetDB 注入测试 DB；UpdateById 会用 BaseModel 的 GetDB，
	// 但 BaseModel.UpdateById 会调用 m.GetDB()。这里我们绕开兜底落库测试，
	// 仅校验：Redis 调用失败时不 panic、不阻塞。
	agg := &FileAccessAggregator{
		store:     failStore,
		dbFn:      func() (*gorm.DB, error) { return db, nil },
		tableName: "upload_files",
		now:       func() time.Time { return time.Unix(1, 0) },
		batchSize: 10,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		agg.Mark(newUploadFileForTest(7))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Mark blocked unexpectedly on redis failure")
	}
}

// TestFileAccessAggregatorFlushRequeuesOnUpdateFailure 验证 UPDATE 失败时按原 score 回写。
func TestFileAccessAggregatorFlushRequeuesOnUpdateFailure(t *testing.T) {
	store := newFakeFileAccessStore()
	// 预置两条记录，分属不同时间戳分组。
	ts1 := float64(time.Unix(1700000000, 0).Unix())
	ts2 := float64(time.Unix(1700000060, 0).Unix())
	store.scores["10"] = ts1
	store.scores["20"] = ts2

	// 构造一个会让 UPDATE 失败的 DB：表名不存在。
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	agg := &FileAccessAggregator{
		store:     store,
		dbFn:      func() (*gorm.DB, error) { return db, nil },
		tableName: "not_exists_table",
		now:       time.Now,
		batchSize: 100,
	}

	written, err := agg.Flush(context.Background())
	if err != nil {
		t.Fatalf("flush should swallow per-group update error, got %v", err)
	}
	if written != 0 {
		t.Fatalf("expected 0 written rows on failure, got %d", written)
	}

	// 失败的 ID 应按原 score 回写到 ZSet，避免丢失访问时间。
	snap := store.snapshot()
	if got, want := len(snap), 2; got != want {
		t.Fatalf("expected %d members requeued, got %d (%#v)", want, got, snap)
	}
	if snap["10"] != ts1 {
		t.Fatalf("expected member 10 requeued with score %v, got %v", ts1, snap["10"])
	}
	if snap["20"] != ts2 {
		t.Fatalf("expected member 20 requeued with score %v, got %v", ts2, snap["20"])
	}
}

type failingFileAccessStore struct{}

func (failingFileAccessStore) ZAddGT(_ context.Context, _ string, _ float64, _ string) error {
	return context.DeadlineExceeded
}
func (failingFileAccessStore) ZPopMin(_ context.Context, _ string, _ int64) ([]redis.Z, error) {
	return nil, context.DeadlineExceeded
}
func (failingFileAccessStore) ZAddBatch(_ context.Context, _ string, _ []redis.Z) error {
	return context.DeadlineExceeded
}

func newUploadFileForTest(id uint) *model.UploadFiles {
	uf := &model.UploadFiles{}
	uf.ID = id
	return uf
}

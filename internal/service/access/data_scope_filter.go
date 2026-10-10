package access

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/data"
	casbinx "github.com/wannanbigpig/gin-layout/internal/access/casbin"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"
)

const (
	// DataScopeAll 全部数据
	DataScopeAll uint8 = 1
	// DataScopeDeptAndChildren 本部门及子级
	DataScopeDeptAndChildren uint8 = 2
	// DataScopeDept 本部门
	DataScopeDept uint8 = 3
	// DataScopeSelf 仅本人
	DataScopeSelf uint8 = 4
	// DataScopeCustom 自定义部门
	DataScopeCustom uint8 = 5

	dataScopeCachePrefix = "data_scope:"
	dataScopeCacheTTL    = 10 * time.Minute
)

// DataScopeInput 数据权限过滤输入。
// 不依赖 *gin.Context，controller 从 AuthPrincipal 转换，worker/cron 从 payload 传入。
type DataScopeInput struct {
	UID          uint
	IsSuperAdmin bool
}

// DataScopeSnapshot 数据权限快照，用于异步导出等脱离 HTTP 上下文的场景。
type DataScopeSnapshot struct {
	ScopeAll bool   `json:"scope_all"`
	DeptIDs  []uint `json:"dept_ids"`
	OwnerUID uint   `json:"owner_uid"`
	DeptCol  string `json:"dept_col"`
	OwnerCol string `json:"owner_col"`
}

// DataScopeFilter 负责构建行级数据权限过滤条件。
// 当前数据权限由业务 service 显式接入；系统管理、日志、文件、任务等治理模块不默认套用。
type DataScopeFilter struct{}

var (
	dataScopeSingleflightGroup  singleflight.Group
	computeScopeForSingleflight = func(f *DataScopeFilter, ctx context.Context, uid uint) (*scopeResult, error) {
		return f.computeScope(ctx, uid)
	}
	currentDataScopeCacheVersion = casbinx.CurrentPermissionVersion
	dataScopeRedisClient         = data.RedisClient
)

// NewDataScopeFilter 创建数据权限过滤器。
func NewDataScopeFilter() *DataScopeFilter {
	return &DataScopeFilter{}
}

// Build 构建行级过滤条件。
// 返回的 cond 可直接拼入 QueryBuilder 或 GORM Where。
// deptCol/ownerCol 为业务表的部门列名和创建者列名。
// 超管短路：返回空条件（不加行级限制）。
// 拿不到 scope 时返回恒假条件 1=0。
func (f *DataScopeFilter) Build(ctx context.Context, input DataScopeInput, deptCol, ownerCol string) (string, []any) {
	if input.IsSuperAdmin {
		return "", nil
	}

	result, err := f.resolveScope(ctx, input.UID)
	if err != nil {
		log.Error("DataScopeFilter: resolve scope failed, denying access",
			zap.Uint("uid", input.UID),
			zap.Error(err))
		return "1=0", nil
	}
	return buildScopeCondition(scopeConditionInput{
		ScopeAll:    result.ScopeAll,
		DeptIDs:     result.DeptIDs,
		OwnerUID:    result.OwnerUID,
		HasDeptIDs:  result.HasDeptIDs,
		HasOwnerUID: result.HasOwnerUID,
		DeptCol:     deptCol,
		OwnerCol:    ownerCol,
	})
}

// ResolveDepartmentScope 返回用户合并后的部门数据范围。
// ScopeAll 为 true 时 DeptIDs 无需参与判断；解析失败时由调用方按 fail-closed 处理。
func (f *DataScopeFilter) ResolveDepartmentScope(ctx context.Context, input DataScopeInput) (bool, []uint, error) {
	if input.IsSuperAdmin {
		return true, nil, nil
	}

	result, err := f.resolveScope(ctx, input.UID)
	if err != nil {
		return false, nil, err
	}
	return result.ScopeAll, append([]uint(nil), result.DeptIDs...), nil
}

func (f *DataScopeFilter) resolveScope(ctx context.Context, uid uint) (*scopeResult, error) {
	// 权限变更可能与当前回源并发；版本变化时最多重试一次，
	// 避免把变更前的计算结果写回并继续用于当前请求。
	for attempt := 0; attempt < 2; attempt++ {
		version, err := currentDataScopeCacheVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("read data scope version: %w", err)
		}
		result, cacheErr := f.getFromCache(ctx, uid, version)
		if cacheErr != nil {
			cacheKey := fmt.Sprintf("%s:%d", f.cacheKey(uid), version)
			v, sfErr, _ := dataScopeSingleflightGroup.Do(cacheKey, func() (interface{}, error) {
				return computeScopeForSingleflight(f, ctx, uid)
			})
			if sfErr != nil {
				return nil, fmt.Errorf("compute data scope for uid %d: %w", uid, sfErr)
			}
			computed, ok := v.(*scopeResult)
			if !ok || computed == nil {
				return nil, fmt.Errorf("invalid data scope result for uid %d", uid)
			}
			result = computed
		}
		if result == nil {
			return nil, fmt.Errorf("empty data scope result for uid %d", uid)
		}
		currentVersion, err := currentDataScopeCacheVersion(ctx)
		if err != nil {
			return nil, fmt.Errorf("recheck data scope version: %w", err)
		}
		if currentVersion != version {
			continue
		}
		if cacheErr != nil {
			f.setToCache(ctx, uid, result, version)
			// 写入期间可能提交撤权；旧版本缓存即使晚到也不能用于本次请求。
			currentVersion, err = currentDataScopeCacheVersion(ctx)
			if err != nil {
				return nil, fmt.Errorf("recheck data scope version after cache write: %w", err)
			}
			if currentVersion != version {
				continue
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("data scope changed repeatedly for uid %d", uid)
}

// BuildFromSnapshot 从快照构建过滤条件，用于异步导出等脱离 HTTP 上下文的场景。
func (f *DataScopeFilter) BuildFromSnapshot(snapshot DataScopeSnapshot) (string, []any) {
	return buildScopeCondition(scopeConditionInput{
		ScopeAll:    snapshot.ScopeAll,
		DeptIDs:     snapshot.DeptIDs,
		OwnerUID:    snapshot.OwnerUID,
		HasDeptIDs:  len(snapshot.DeptIDs) > 0,
		HasOwnerUID: snapshot.OwnerUID > 0,
		DeptCol:     snapshot.DeptCol,
		OwnerCol:    snapshot.OwnerCol,
	})
}

type scopeConditionInput struct {
	ScopeAll    bool
	DeptIDs     []uint
	OwnerUID    uint
	HasDeptIDs  bool
	HasOwnerUID bool
	DeptCol     string
	OwnerCol    string
}

func buildScopeCondition(input scopeConditionInput) (string, []any) {
	if input.ScopeAll {
		return "", nil
	}

	if !input.HasDeptIDs && !input.HasOwnerUID {
		return "1=0", nil
	}

	if input.HasOwnerUID && !input.HasDeptIDs {
		return fmt.Sprintf("%s = ?", input.OwnerCol), []any{input.OwnerUID}
	}

	if input.HasDeptIDs && !input.HasOwnerUID {
		return fmt.Sprintf("%s IN (?)", input.DeptCol), []any{input.DeptIDs}
	}

	return fmt.Sprintf("(%s IN (?) OR %s = ?)", input.DeptCol, input.OwnerCol), []any{input.DeptIDs, input.OwnerUID}
}

// scopeResult 内部计算结果。
type scopeResult struct {
	ScopeAll    bool   `json:"scope_all"`
	DeptIDs     []uint `json:"dept_ids"`
	OwnerUID    uint   `json:"owner_uid"`
	HasDeptIDs  bool   `json:"has_dept_ids"`
	HasOwnerUID bool   `json:"has_owner_uid"`
}

// computeScope 从数据库计算用户的数据权限范围。
func (f *DataScopeFilter) computeScope(ctx context.Context, uid uint) (*scopeResult, error) {
	// 1. 查用户直接角色 ID
	directRoleIDs, err := model.NewAdminUserRoleMap().RoleIdsByUid(uid)
	if err != nil {
		return nil, fmt.Errorf("query direct roles: %w", err)
	}

	// 2. 查用户部门 → 部门角色 ID
	deptIDs, err := model.NewAdminUserDeptMap().DeptIdsByUid(uid)
	if err != nil {
		return nil, fmt.Errorf("query user depts: %w", err)
	}
	var deptRoleIDs []uint
	if len(deptIDs) > 0 {
		deptRoleIDs, err = model.NewDeptRoleMap().RoleIdsByDeptIds(deptIDs)
		if err != nil {
			return nil, fmt.Errorf("query dept roles: %w", err)
		}
	}

	// 3. 合并角色 ID 并去重
	allRoleIDs := UniqueUintSlice(append(directRoleIDs, deptRoleIDs...))
	if len(allRoleIDs) == 0 {
		return &scopeResult{}, nil
	}

	// 4. 查询启用角色的 data_scope
	enabledRoleIDs, err := model.NewRole().EnabledIdsByIds(allRoleIDs)
	if err != nil {
		return nil, fmt.Errorf("query enabled roles: %w", err)
	}
	if len(enabledRoleIDs) == 0 {
		return &scopeResult{}, nil
	}

	roles, err := f.loadRoles(enabledRoleIDs)
	if err != nil {
		return nil, fmt.Errorf("load roles: %w", err)
	}

	// 5. 按 data_scope 分组计算
	result := &scopeResult{
		OwnerUID: uid, // 默认设置，仅在 scope=4 时使用
	}

	scopeDeptIDs := make(map[uint8][]uint)
	for _, role := range roles {
		scopeDeptIDs[role.DataScope] = append(scopeDeptIDs[role.DataScope], role.ID)
	}

	// 任一角色为全部数据，直接放行
	if _, ok := scopeDeptIDs[DataScopeAll]; ok {
		result.ScopeAll = true
		return result, nil
	}

	// 收集部门 ID
	var mergedDeptIDs []uint
	hasDeptScope := false
	hasSelfScope := false

	// 本部门及子级
	if roleIDs, ok := scopeDeptIDs[DataScopeDeptAndChildren]; ok && len(roleIDs) > 0 {
		hasDeptScope = true
		subDeptIDs, err := f.collectDeptAndChildren(deptIDs)
		if err != nil {
			return nil, fmt.Errorf("collect dept children: %w", err)
		}
		mergedDeptIDs = append(mergedDeptIDs, subDeptIDs...)
	}

	// 本部门
	if roleIDs, ok := scopeDeptIDs[DataScopeDept]; ok && len(roleIDs) > 0 {
		hasDeptScope = true
		mergedDeptIDs = append(mergedDeptIDs, deptIDs...)
	}

	// 自定义部门
	if roleIDs, ok := scopeDeptIDs[DataScopeCustom]; ok && len(roleIDs) > 0 {
		hasDeptScope = true
		customDeptIDs, err := model.NewRoleDataScopeDept().DeptIdsByRoleIds(roleIDs)
		if err != nil {
			return nil, fmt.Errorf("query custom dept scope: %w", err)
		}
		mergedDeptIDs = append(mergedDeptIDs, customDeptIDs...)
	}

	// 仅本人
	if _, ok := scopeDeptIDs[DataScopeSelf]; ok {
		hasSelfScope = true
	}

	if hasDeptScope && len(mergedDeptIDs) > 0 {
		result.DeptIDs = UniqueUintSlice(mergedDeptIDs)
		result.HasDeptIDs = true
	}

	if hasSelfScope {
		result.HasOwnerUID = true
	}

	return result, nil
}

// loadRoles 批量加载角色信息。
func (f *DataScopeFilter) loadRoles(roleIDs []uint) ([]*model.Role, error) {
	db, err := model.GetDB(model.NewRole())
	if err != nil {
		return nil, err
	}
	var roles []*model.Role
	if err := db.Where("id IN ?", roleIDs).Select("id", "data_scope").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// collectDeptAndChildren 收集直属部门及其所有子部门 ID。
func (f *DataScopeFilter) collectDeptAndChildren(deptIDs []uint) ([]uint, error) {
	if len(deptIDs) == 0 {
		return nil, nil
	}
	db, err := model.GetDB(model.NewDepartment())
	if err != nil {
		return nil, err
	}
	return collectDeptAndChildrenFromDB(db, deptIDs)
}

func collectDeptAndChildrenFromDB(db *gorm.DB, deptIDs []uint) ([]uint, error) {
	roots := UniqueUintSlice(deptIDs)
	if len(roots) == 0 {
		return nil, nil
	}

	allIDs := make([]uint, 0, len(roots))
	seen := make(map[uint]struct{}, len(roots))
	frontier := make([]uint, 0, len(roots))
	for _, id := range roots {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		allIDs = append(allIDs, id)
		frontier = append(frontier, id)
	}

	for len(frontier) > 0 {
		var children []uint
		if err := db.Model(&model.Department{}).Where("pid IN ?", frontier).Pluck("id", &children).Error; err != nil {
			return nil, err
		}

		next := make([]uint, 0, len(children))
		for _, childID := range UniqueUintSlice(children) {
			if childID == 0 {
				continue
			}
			if _, ok := seen[childID]; ok {
				continue
			}
			seen[childID] = struct{}{}
			allIDs = append(allIDs, childID)
			next = append(next, childID)
		}
		frontier = next
	}

	return allIDs, nil
}

// cacheEntry 缓存条目。
type cacheEntry struct {
	Result   *scopeResult `json:"result"`
	Version  uint64       `json:"version"`
	CachedAt time.Time    `json:"cached_at"`
}

// cacheKey 生成缓存 key。
func (f *DataScopeFilter) cacheKey(uid uint) string {
	return redisutil.Channel(fmt.Sprintf("%s%d", dataScopeCachePrefix, uid))
}

// getFromCache 从 Redis 读取缓存。
func (f *DataScopeFilter) getFromCache(ctx context.Context, uid uint, version uint64) (*scopeResult, error) {
	client := dataScopeRedisClient()
	if client == nil {
		return nil, fmt.Errorf("redis unavailable")
	}

	val, err := client.Get(ctx, f.cacheKey(uid)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("cache miss")
		}
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, err
	}
	if entry.Version != version {
		return nil, fmt.Errorf("stale cache version")
	}
	return entry.Result, nil
}

// setToCache 写入 Redis 缓存。
func (f *DataScopeFilter) setToCache(ctx context.Context, uid uint, result *scopeResult, version uint64) {
	client := dataScopeRedisClient()
	if client == nil {
		return
	}

	entry := cacheEntry{
		Result:   result,
		Version:  version,
		CachedAt: time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		log.Error("DataScopeFilter: marshal cache entry failed", zap.Uint("uid", uid), zap.Error(err))
		return
	}

	jitter := time.Duration(rand.Intn(30)) * time.Second
	ttl := dataScopeCacheTTL + jitter
	if err := client.Set(ctx, f.cacheKey(uid), data, ttl).Err(); err != nil {
		log.Error("DataScopeFilter: set cache failed", zap.Uint("uid", uid), zap.Error(err))
	}
}

// InvalidateCache 使指定用户的数据权限缓存失效。
// 应在以下场景调用：角色 data_scope/自定义部门、用户部门、用户角色、部门角色、部门树结构变化。
// 权限变更事务必须已推进共享版本；删除只是清理，版本复核才是并发失效边界。
func (f *DataScopeFilter) InvalidateCache(ctx context.Context, uids ...uint) {
	uids = UniqueUintSlice(uids)

	client := dataScopeRedisClient()
	if client == nil || len(uids) == 0 {
		return
	}

	keys := make([]string, 0, len(uids))
	for _, uid := range uids {
		keys = append(keys, f.cacheKey(uid))
	}

	if err := client.Del(ctx, keys...).Err(); err != nil {
		log.Error("DataScopeFilter: invalidate cache failed", zap.Uints("uids", uids), zap.Error(err))
	}
}

// InvalidateAllCache 使所有用户的数据权限缓存失效。
// 部门树结构变化会影响多个祖先部门的子级范围，该场景优先保证正确性。
func (f *DataScopeFilter) InvalidateAllCache(ctx context.Context) {
	client := dataScopeRedisClient()
	if client == nil {
		return
	}

	var cursor uint64
	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, redisutil.Channel(dataScopeCachePrefix)+"*", 100).Result()
		if err != nil {
			log.Error("DataScopeFilter: scan cache keys failed", zap.Error(err))
			return
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				log.Error("DataScopeFilter: invalidate all cache entries failed", zap.Error(err))
				return
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			return
		}
	}
}

package dashboard

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
)

// MonitorSnapshot 服务器监控快照。
type MonitorSnapshot struct {
	Host    HostInfo    `json:"host"`
	Runtime RuntimeInfo `json:"runtime"`
	App     AppInfo     `json:"app"`
}

// HostInfo 主机资源信息。
type HostInfo struct {
	CPU       CPUInfo    `json:"cpu"`
	Memory    MemoryInfo `json:"memory"`
	Disk      DiskInfo   `json:"disk"`
	Load      LoadInfo   `json:"load"`
	Uptime    uint64     `json:"uptime"`
	Hostname  string     `json:"hostname"`
	OS        string     `json:"os"`
	Platform  string     `json:"platform"`
	UpdatedAt string     `json:"updated_at"`
}

// CPUInfo CPU 信息。
type CPUInfo struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

// MemoryInfo 内存信息。
type MemoryInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Available    uint64  `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskInfo 磁盘信息（根分区）。
type DiskInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// LoadInfo 系统负载。
type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// RuntimeInfo Go 运行时信息。
type RuntimeInfo struct {
	Goroutines   int    `json:"goroutines"`
	HeapAlloc    uint64 `json:"heap_alloc"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapReleased uint64 `json:"heap_released"`
	GCCycles     uint32 `json:"gc_cycles"`
	GCPauseTotal uint64 `json:"gc_pause_total_ns"`
	GoVersion    string `json:"go_version"`
}

// AppInfo 应用态信息。
type AppInfo struct {
	OnlineSessions int64                        `json:"online_sessions"`
	WSOnline       int                          `json:"ws_online"`
	QueuePending   int64                        `json:"queue_pending"`
	QueueRetrying  int64                        `json:"queue_retrying"`
	QueueRunning   int64                        `json:"queue_running"`
	QueueStats     []*queue.QueueStats          `json:"queue_stats"`
	Audit          audit.RuntimeMetricsSnapshot `json:"audit"`
}

// MonitorService 服务器监控服务。
type MonitorService struct{}

func NewMonitorService() *MonitorService {
	return &MonitorService{}
}

// GetSnapshot 获取服务器监控快照。
func (s *MonitorService) GetSnapshot() (*MonitorSnapshot, error) {
	hostInfo := s.collectHostInfo()
	runtimeInfo := s.collectRuntimeInfo()
	appInfo := s.collectAppInfo()

	return &MonitorSnapshot{
		Host:    hostInfo,
		Runtime: runtimeInfo,
		App:     appInfo,
	}, nil
}

func (s *MonitorService) collectHostInfo() HostInfo {
	info := HostInfo{
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	// CPU
	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		info.CPU.UsagePercent = cpuPercent[0]
	}
	if cores, err := cpu.Counts(true); err == nil {
		info.CPU.Cores = cores
	}

	// Memory
	if vm, err := mem.VirtualMemory(); err == nil {
		info.Memory = MemoryInfo{
			Total:        vm.Total,
			Used:         vm.Used,
			Available:    vm.Available,
			UsagePercent: vm.UsedPercent,
		}
	}

	// Disk（根分区）
	if usage, err := disk.Usage("/"); err == nil {
		info.Disk = DiskInfo{
			Total:        usage.Total,
			Used:         usage.Used,
			Free:         usage.Free,
			UsagePercent: usage.UsedPercent,
		}
	}

	// Load
	if avg, err := load.Avg(); err == nil {
		info.Load = LoadInfo{
			Load1:  avg.Load1,
			Load5:  avg.Load5,
			Load15: avg.Load15,
		}
	}

	// Host
	if uptime, err := host.Uptime(); err == nil {
		info.Uptime = uptime
	}
	if hostname, err := host.Info(); err == nil {
		info.Hostname = hostname.Hostname
		info.OS = hostname.OS
		info.Platform = hostname.Platform
	}

	return info
}

func (s *MonitorService) collectRuntimeInfo() RuntimeInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return RuntimeInfo{
		Goroutines:   runtime.NumGoroutine(),
		HeapAlloc:    m.HeapAlloc,
		HeapInuse:    m.HeapInuse,
		HeapSys:      m.HeapSys,
		HeapReleased: m.HeapReleased,
		GCCycles:     m.NumGC,
		GCPauseTotal: m.PauseTotalNs,
		GoVersion:    runtime.Version(),
	}
}

func (s *MonitorService) collectAppInfo() AppInfo {
	info := AppInfo{Audit: audit.RuntimeMetrics()}

	// 在线会话数（复用 session 统计）
	info.OnlineSessions = countOnlineSessions()

	// WS 在线连接数
	info.WSOnline = notification.OnlineCount()

	// 队列积压（从 Asynq Inspector 读取原始 stats）
	collectQueueStats(&info)

	return info
}

func countOnlineSessions() int64 {
	db, err := model.GetDB()
	if err != nil {
		return 0
	}
	var count int64
	if err := db.Table("admin_login_logs").
		Where("login_status = ? AND is_revoked = ? AND token_expires > ?", model.LoginStatusSuccess, model.IsRevokedNo, time.Now()).
		Count(&count).Error; err != nil {
		log.Warn("统计在线会话数失败", zap.Error(err))
		return 0
	}
	return count
}

func collectQueueStats(info *AppInfo) {
	ctx := context.Background()
	stats, err := queue.GetAllQueueStats(ctx)
	if err != nil {
		log.Warn("获取队列统计失败，降级为 DB 统计", zap.Error(err))
		countQueueBacklogFromDB(info)
		return
	}

	info.QueueStats = stats
	var pending, retry, running int64
	for _, s := range stats {
		pending += int64(s.Pending)
		retry += int64(s.Retry)
		running += int64(s.Active)
	}
	info.QueuePending = pending
	info.QueueRetrying = retry
	info.QueueRunning = running
}

// countQueueBacklogFromDB 降级方案：从 DB 的 task_run 表统计队列积压。
func countQueueBacklogFromDB(info *AppInfo) {
	db, err := model.GetDB()
	if err != nil {
		return
	}

	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	if err := db.Table("task_runs").
		Select("status, COUNT(*) AS count").
		Where("status IN ?", []string{model.TaskRunStatusPending, model.TaskRunStatusRetrying, model.TaskRunStatusRunning}).
		Group("status").
		Scan(&rows).Error; err != nil {
		log.Warn("统计队列积压失败", zap.Error(err))
		return
	}

	for _, row := range rows {
		switch row.Status {
		case model.TaskRunStatusPending:
			info.QueuePending = row.Count
		case model.TaskRunStatusRetrying:
			info.QueueRetrying = row.Count
		case model.TaskRunStatusRunning:
			info.QueueRunning = row.Count
		}
	}
}

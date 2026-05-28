package taskcron

import (
	"context"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"go.uber.org/zap"
)

var (
	cleanExpiredFilesGetDB     = model.GetDB
	cleanExpiredFilesDestroy   = func(id uint) error { return service.NewFileResourceService().Destroy(id) }
	cleanExpiredFilesBatchSize = 100
)

func CleanExpiredFilesHandler(ctx context.Context, _ map[string]any) error {
	log.Logger.Info("开始执行清理过期文件任务")
	db, err := cleanExpiredFilesGetDB()
	if err != nil {
		return err
	}

	now := time.Now()
	var lastID uint
	successCount := 0
	failCount := 0
	totalScanned := 0

	// 基于 ID 游标的分批扫描，避免一次性加载全部过期文件导致 OOM
	for {
		var files []model.UploadFiles
		err = db.Unscoped().Model(&model.UploadFiles{}).
			Where("id > ? AND expires_at IS NOT NULL AND expires_at <= ?", lastID, now).
			Order("id ASC").
			Limit(cleanExpiredFilesBatchSize).
			Find(&files).Error
		if err != nil {
			log.Logger.Error("查询过期文件失败", zap.Error(err))
			return err
		}

		if len(files) == 0 {
			break
		}

		totalScanned += len(files)
		for _, file := range files {
			if file.ID > lastID {
				lastID = file.ID
			}
			// 物理删除，联动清理关联引用与物理介质
			if destroyErr := cleanExpiredFilesDestroy(file.ID); destroyErr != nil {
				log.Logger.Error("物理清理过期文件失败", zap.Uint("id", file.ID), zap.String("name", file.OriginName), zap.Error(destroyErr))
				failCount++
			} else {
				log.Logger.Info("物理清理过期文件成功", zap.Uint("id", file.ID), zap.String("name", file.OriginName))
				successCount++
			}
		}

		// 不足一批说明已扫描完毕
		if len(files) < cleanExpiredFilesBatchSize {
			break
		}
	}

	if totalScanned == 0 {
		log.Logger.Info("没有发现过期的临时文件")
		return nil
	}

	log.Logger.Info("清理过期文件完成", zap.Int("scanned", totalScanned), zap.Int("success", successCount), zap.Int("failed", failCount))
	return nil
}

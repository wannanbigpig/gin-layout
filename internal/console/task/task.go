package task

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/config"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/queue"
)

type asyncScanRow struct {
	TaskType  string
	InBuiltin bool
	InDB      bool
	Queue     string
}

var (
	Cmd = &cobra.Command{
		Use:   "task",
		Short: "Task helper commands",
	}

	scanAsyncCmd = &cobra.Command{
		Use:   "scan-async",
		Short: "Scan registered async queue tasks and compare definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScanAsync()
		},
	}

	scanCronCmd = &cobra.Command{
		Use:   "scan-cron",
		Short: "Scan built-in cron tasks and compare definitions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScanCron()
		},
	}
)

func init() {
	Cmd.AddCommand(scanAsyncCmd)
	Cmd.AddCommand(scanCronCmd)
}

func runScanAsync() error {
	taskTypes := collectRegistryTaskTypes(jobs.NewRegistry())
	if len(taskTypes) == 0 {
		fmt.Println("未扫描到已注册的异步任务。")
		return nil
	}

	builtinMap := collectBuiltinDefinitionsByKind(model.TaskKindAsync)
	dbMap, dbReady, dbErr := loadDBDefinitionsByKind(model.TaskKindAsync)
	if dbErr != nil {
		return dbErr
	}

	rows := buildAsyncScanRows(taskTypes, builtinMap, dbMap, dbReady)
	printScanRows(rows, dbReady)
	printScanSummary("代码注册异步任务", taskTypes, rows, builtinMap, dbMap, dbReady)
	return nil
}

func runScanCron() error {
	builtinMap := collectBuiltinDefinitionsByKind(model.TaskKindCron)
	taskTypes := sortedDefinitionCodes(builtinMap)
	if len(taskTypes) == 0 {
		fmt.Println("未扫描到内置定时任务。")
		return nil
	}

	dbMap, dbReady, dbErr := loadDBDefinitionsByKind(model.TaskKindCron)
	if dbErr != nil {
		return dbErr
	}

	rows := buildAsyncScanRows(taskTypes, builtinMap, dbMap, dbReady)
	printScanRows(rows, dbReady)
	printScanSummary("内置定时任务", taskTypes, rows, builtinMap, dbMap, dbReady)
	return nil
}

func collectRegistryTaskTypes(registry queue.Registry) []string {
	if registry == nil {
		return nil
	}
	entries := registry.Entries()
	taskTypeSet := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		taskType := strings.TrimSpace(entry.TaskType)
		if taskType == "" {
			continue
		}
		taskTypeSet[taskType] = struct{}{}
	}
	taskTypes := make([]string, 0, len(taskTypeSet))
	for taskType := range taskTypeSet {
		taskTypes = append(taskTypes, taskType)
	}
	sort.Strings(taskTypes)
	return taskTypes
}

func collectBuiltinDefinitionsByKind(kind string) map[string]model.TaskDefinition {
	definitions := taskcron.BuiltinTaskDefinitions(config.GetConfig())
	result := make(map[string]model.TaskDefinition, len(definitions))
	for _, definition := range definitions {
		if definition.Kind != kind {
			continue
		}
		code := strings.TrimSpace(definition.Code)
		if code == "" {
			continue
		}
		result[code] = definition
	}
	return result
}

func sortedDefinitionCodes(definitions map[string]model.TaskDefinition) []string {
	codes := make([]string, 0, len(definitions))
	for code := range definitions {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func loadDBDefinitionsByKind(kind string) (map[string]model.TaskDefinition, bool, error) {
	db, err := model.GetDB()
	if err != nil {
		return map[string]model.TaskDefinition{}, false, nil
	}
	if !db.Migrator().HasTable(model.NewTaskDefinition().TableName()) {
		return map[string]model.TaskDefinition{}, false, nil
	}

	definitions := make([]model.TaskDefinition, 0)
	if err := db.Where("kind = ? AND deleted_at = 0", kind).Find(&definitions).Error; err != nil {
		return nil, true, err
	}

	result := make(map[string]model.TaskDefinition, len(definitions))
	for _, definition := range definitions {
		code := strings.TrimSpace(definition.Code)
		if code == "" {
			continue
		}
		result[code] = definition
	}
	return result, true, nil
}

func buildAsyncScanRows(taskTypes []string, builtinMap map[string]model.TaskDefinition, dbMap map[string]model.TaskDefinition, dbReady bool) []asyncScanRow {
	rows := make([]asyncScanRow, 0, len(taskTypes))
	for _, taskType := range taskTypes {
		row := asyncScanRow{
			TaskType:  taskType,
			InBuiltin: false,
			InDB:      false,
			Queue:     "-",
		}
		if definition, ok := builtinMap[taskType]; ok {
			row.InBuiltin = true
			if strings.TrimSpace(definition.Queue) != "" {
				row.Queue = definition.Queue
			}
		}
		if dbReady {
			if definition, ok := dbMap[taskType]; ok {
				row.InDB = true
				if row.Queue == "-" && strings.TrimSpace(definition.Queue) != "" {
					row.Queue = definition.Queue
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func printScanRows(rows []asyncScanRow, dbReady bool) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if dbReady {
		_, _ = fmt.Fprintln(writer, "TASK_TYPE\tIN_BUILTIN\tIN_DB\tQUEUE")
	} else {
		_, _ = fmt.Fprintln(writer, "TASK_TYPE\tIN_BUILTIN\tQUEUE")
	}
	for _, row := range rows {
		if dbReady {
			_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n",
				row.TaskType, yesNo(row.InBuiltin), yesNo(row.InDB), row.Queue)
		} else {
			_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\n",
				row.TaskType, yesNo(row.InBuiltin), row.Queue)
		}
	}
	_ = writer.Flush()
}

func printScanSummary(sourceLabel string, taskTypes []string, rows []asyncScanRow, builtinMap map[string]model.TaskDefinition, dbMap map[string]model.TaskDefinition, dbReady bool) {
	missingBuiltin := make([]string, 0)
	missingDB := make([]string, 0)
	for _, row := range rows {
		if !row.InBuiltin {
			missingBuiltin = append(missingBuiltin, row.TaskType)
		}
		if dbReady && !row.InDB {
			missingDB = append(missingDB, row.TaskType)
		}
	}

	staleBuiltin := make([]string, 0)
	for code := range builtinMap {
		if !contains(taskTypes, code) {
			staleBuiltin = append(staleBuiltin, code)
		}
	}
	sort.Strings(staleBuiltin)

	staleDB := make([]string, 0)
	if dbReady {
		for code := range dbMap {
			if !contains(taskTypes, code) {
				staleDB = append(staleDB, code)
			}
		}
		sort.Strings(staleDB)
	}

	fmt.Printf("\n%s: %d\n", sourceLabel, len(taskTypes))
	fmt.Printf("缺失内置定义: %d\n", len(missingBuiltin))
	if dbReady {
		fmt.Printf("缺失数据库定义: %d\n", len(missingDB))
	} else {
		fmt.Println("数据库定义状态: 未连接或未初始化，已跳过 IN_DB 对比")
	}
	fmt.Printf("内置定义疑似冗余: %d\n", len(staleBuiltin))
	if dbReady {
		fmt.Printf("数据库定义疑似冗余: %d\n", len(staleDB))
	}

	if len(missingBuiltin) > 0 {
		fmt.Printf("缺失内置定义任务: %s\n", strings.Join(missingBuiltin, ", "))
	}
	if dbReady && len(missingDB) > 0 {
		fmt.Printf("缺失数据库定义任务: %s\n", strings.Join(missingDB, ", "))
	}
	if len(staleBuiltin) > 0 {
		fmt.Printf("内置定义冗余任务: %s\n", strings.Join(staleBuiltin, ", "))
	}
	if dbReady && len(staleDB) > 0 {
		fmt.Printf("数据库定义冗余任务: %s\n", strings.Join(staleDB, ", "))
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func yesNo(value bool) string {
	if value {
		return "Y"
	}
	return "N"
}

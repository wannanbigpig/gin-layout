package taskcenter

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
)

const (
	TaskDefinitionChangeChannel = "task_definition:changed"
	taskDefinitionChangeTimeout = 3 * time.Second
)

type TaskDefinitionChangePayload struct {
	Source    string `json:"source"`
	TaskCode  string `json:"task_code"`
	Timestamp int64  `json:"timestamp"`
}

var taskDefinitionChangeSourceID = buildTaskDefinitionChangeSourceID()

func NotifyTaskDefinitionChanged(taskCode string) error {
	cfg := config.GetConfig()
	if cfg == nil || !cfg.Redis.Enable {
		return nil
	}
	client := data.RedisClient()
	if client == nil {
		return nil
	}

	payload, err := json.Marshal(TaskDefinitionChangePayload{
		Source:    taskDefinitionChangeSourceID,
		TaskCode:  strings.TrimSpace(taskCode),
		Timestamp: time.Now().Unix(),
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), taskDefinitionChangeTimeout)
	defer cancel()
	return client.Publish(ctx, redisutil.Channel(TaskDefinitionChangeChannel), payload).Err()
}

func DecodeTaskDefinitionChangePayload(raw string) (TaskDefinitionChangePayload, bool) {
	payload := TaskDefinitionChangePayload{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return payload, true
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return payload, false
	}
	return payload, true
}

func buildTaskDefinitionChangeSourceID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return host + ":" + strconv.Itoa(os.Getpid())
}

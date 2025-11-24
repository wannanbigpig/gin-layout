package captcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/steambap/captcha"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

type Item struct {
	Id     string `json:"id"`
	B64s   string `json:"b64s"`
	Answer string `json:"answer"`
}

// 内存存储（当 Redis 不可用时使用）
type memoryStore struct {
	data map[string]string
	mu   sync.RWMutex
}

var memStore = &memoryStore{
	data: make(map[string]string),
}

func (m *memoryStore) Set(id, answer string, expiration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = answer
	// 简单的过期处理：启动一个 goroutine 在过期后删除
	go func() {
		time.Sleep(expiration)
		m.mu.Lock()
		delete(m.data, id)
		m.mu.Unlock()
	}()
}

func (m *memoryStore) Get(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	answer, ok := m.data[id]
	return answer, ok
}

func (m *memoryStore) Delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
}

const (
	// captchaRedisKeyPrefix Redis key 前缀
	captchaRedisKeyPrefix = "captcha:"
	// captchaExpiration 验证码过期时间（5分钟）
	captchaExpiration = 5 * time.Minute
)

// setCaptchaAnswer 存储验证码答案
func setCaptchaAnswer(id, answer string) {
	if config.Config.Redis.Enable && data.RedisClient() != nil {
		// 使用 Redis 存储
		ctx := context.Background()
		key := captchaRedisKeyPrefix + id
		_ = data.RedisClient().Set(ctx, key, answer, captchaExpiration).Err()
	} else {
		// 使用内存存储
		memStore.Set(id, answer, captchaExpiration)
	}
}

// getCaptchaAnswer 获取验证码答案
func getCaptchaAnswer(id string) (string, bool) {
	if config.Config.Redis.Enable && data.RedisClient() != nil {
		// 从 Redis 获取
		ctx := context.Background()
		key := captchaRedisKeyPrefix + id
		answer, err := data.RedisClient().Get(ctx, key).Result()
		if err != nil {
			return "", false
		}
		return answer, true
	}
	// 从内存获取
	return memStore.Get(id)
}

// deleteCaptchaAnswer 删除验证码答案（验证后删除）
func deleteCaptchaAnswer(id string) {
	if config.Config.Redis.Enable && data.RedisClient() != nil {
		// 从 Redis 删除
		ctx := context.Background()
		key := captchaRedisKeyPrefix + id
		_ = data.RedisClient().Del(ctx, key).Err()
	} else {
		// 从内存删除
		memStore.Delete(id)
	}
}

// Generate 创建验证码
// 返回验证码 ID、base64 编码的图片和答案（本地环境返回答案，其他环境不返回）
// 验证码为4位字母数字混合
func Generate() (item *Item, err error) {
	// 创建验证码图片，4位字母数字
	img, err := captcha.New(200, 80, func(options *captcha.Options) {
		// 设置字符集：大小写字母和数字
		options.CharPreset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		// 设置验证码长度为4位
		options.TextLength = 4
		// 设置字体缩放
		options.FontScale = 1.2
		// 设置干扰线数量
		options.CurveNumber = 2
		// 设置干扰点数量
		options.Noise = 0.1
	})
	if err != nil {
		return nil, errors.NewBusinessError(1, "Failed to generate verification code")
	}

	// 创建图片缓冲区
	var buf bytes.Buffer

	// 将图片编码为 PNG 并写入缓冲区
	err = img.WriteImage(&buf)
	if err != nil {
		return nil, errors.NewBusinessError(1, "Failed to encode verification code image")
	}

	// 将图片编码为 base64
	b64s := base64.StdEncoding.EncodeToString(buf.Bytes())
	// 添加 data URI 前缀
	b64s = "data:image/png;base64," + b64s

	// 生成唯一的验证码 ID
	captchaID := uuid.New().String()

	// 存储验证码答案
	setCaptchaAnswer(captchaID, img.Text)

	// 获取验证码答案（仅用于本地/测试环境）
	var answer string
	if config.Config.AppEnv == "local" || config.Config.AppEnv == "test" {
		answer = img.Text
	}

	return &Item{
		Id:     captchaID,
		B64s:   b64s,
		Answer: answer,
	}, nil
}

// Verify 校验验证码
func Verify(id, value string) bool {
	// 获取存储的验证码答案
	answer, ok := getCaptchaAnswer(id)
	if !ok {
		return false
	}

	// 比较验证码（不区分大小写）
	if !strings.EqualFold(answer, value) {
		return false
	}

	// 验证成功后删除验证码（一次性使用）
	deleteCaptchaAnswer(id)

	return true
}

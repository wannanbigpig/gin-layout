package captcha

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
	"uuid"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
)

type Item struct {
	Id     string `json:"id"`
	B64s   string `json:"b64s"`
	Answer string `json:"answer"`
}

// 内存存储（未启用 Redis 时使用）
type memoryStore struct {
	data map[string]string
	mu   sync.RWMutex
}

var memStore = &memoryStore{
	data: make(map[string]string),
}

// captchaInstance 验证码实例
var captchaInstance *base64Captcha.Captcha
var captchaOnce sync.Once
var captchaBaseContext = context.Background

func (m *memoryStore) Set(id, answer string, expiration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id] = answer
	// 使用 time.AfterFunc 替代 goroutine + sleep，避免 goroutine 泄漏
	time.AfterFunc(expiration, func() {
		m.mu.Lock()
		delete(m.data, id)
		m.mu.Unlock()
	})
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

func (m *memoryStore) VerifyAndDelete(id, value string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	answer, ok := m.data[id]
	if !ok || !equalIgnoreCase(answer, value) {
		return false
	}
	delete(m.data, id)
	return true
}

const (
	// captchaRedisKeyPrefix Redis key 前缀
	captchaRedisKeyPrefix = "captcha:"
	// captchaExpiration 验证码过期时间（5分钟）
	captchaExpiration = 5 * time.Minute
	// captchaLength 验证码长度
	captchaLength = 4
	// captchaRedisTimeout Redis 操作超时时间
	captchaRedisTimeout = 2 * time.Second
	// captchaCharset 验证码字符集：使用库提供的字符集，避免乱码
	// 组合字母和数字，排除容易混淆的字符（如 0/O, 1/l/I）
	captchaCharset = base64Captcha.TxtAlphabet + base64Captcha.TxtNumbers
)

func withRedisTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(captchaBaseContext(), captchaRedisTimeout)
}

// initCaptcha 初始化验证码实例
func initCaptcha() {
	captchaOnce.Do(func() {
		// 创建字母数字混合验证码驱动
		// 使用 NewDriverString 支持自定义字符集
		// 参数：高度80，宽度240，干扰线数量2，显示选项，长度4，字符集
		driver := base64Captcha.NewDriverString(
			80,  // 高度
			240, // 宽度
			2,   // 干扰线数量
			base64Captcha.OptionShowHollowLine|base64Captcha.OptionShowSlimeLine, // 显示选项
			captchaLength,                      // 长度
			captchaCharset,                     // 字符集（字母数字混合）
			nil,                                // 背景色（nil 使用默认）
			base64Captcha.DefaultEmbeddedFonts, // 字体存储
			nil,                                // 字体列表（nil 使用默认字体）
		)

		// 创建内存存储
		store := base64Captcha.NewMemoryStore(1000, captchaExpiration)

		// 创建验证码实例
		captchaInstance = base64Captcha.NewCaptcha(driver, store)
	})
}

// setCaptchaAnswer 存储验证码答案
func setCaptchaAnswer(id, answer string) error {
	cfg := config.GetConfig()
	if cfg != nil && cfg.Redis.Enable {
		redisClient := data.RedisClient()
		if redisClient == nil {
			if err := data.GetRedisInitError(); err != nil {
				return fmt.Errorf("store captcha answer in redis: %w", err)
			}
			return fmt.Errorf("store captcha answer in redis: %w", data.ErrRedisUnavailable)
		}
		return setCaptchaAnswerInRedis(redisClient, id, answer)
	}
	memStore.Set(id, answer, captchaExpiration)
	return nil
}

func setCaptchaAnswerInRedis(redisClient *redis.Client, id, answer string) error {
	if redisClient == nil {
		return data.ErrRedisUnavailable
	}
	ctx, cancel := withRedisTimeout()
	defer cancel()
	key := captchaRedisKeyPrefix + id
	if err := redisClient.Set(ctx, key, answer, captchaExpiration).Err(); err != nil {
		return fmt.Errorf("set captcha answer: %w", err)
	}
	return nil
}

// verifyAndConsumeCaptchaAnswer 原子校验并消费验证码。
// Redis 模式不依赖生成验证码的进程内存，可用于负载均衡后的多实例校验。
func verifyAndConsumeCaptchaAnswer(id, value string) bool {
	cfg := config.GetConfig()
	redisClient := data.RedisClient()
	if cfg != nil && cfg.Redis.Enable && redisClient != nil {
		ctx, cancel := withRedisTimeout()
		defer cancel()
		key := captchaRedisKeyPrefix + id
		const script = `local answer=redis.call('GET',KEYS[1]); if not answer then return 0 end; if string.lower(answer)~=string.lower(ARGV[1]) then return 0 end; redis.call('DEL',KEYS[1]); return 1`
		verified, err := redisClient.Eval(ctx, script, []string{key}, value).Int()
		return err == nil && verified == 1
	}
	return memStore.VerifyAndDelete(id, value)
}

// Generate 创建验证码
// 返回验证码 ID、base64 编码的图片和答案（本地环境返回答案，其他环境不返回）
// 验证码为4位字母数字混合
func Generate() (item *Item, err error) {
	return generateWithStore(setCaptchaAnswer)
}

func generateWithStore(storeAnswer func(string, string) error) (item *Item, err error) {
	// 初始化验证码实例
	initCaptcha()

	// 生成唯一的验证码 ID（我们使用 UUID）
	captchaID := uuid.New().String()

	// 生成验证码（返回内部 ID、base64 编码的图片、答案和可能的错误）
	_, b64s, answer, err := captchaInstance.Generate()
	if err != nil {
		return nil, err
	}

	// 存储验证码答案（使用我们的 UUID 作为 key，存储实际的验证码文本）
	if err := storeAnswer(captchaID, answer); err != nil {
		return nil, err
	}

	// 添加 data URI 前缀（base64Captcha 已经返回了 base64 字符串）
	if len(b64s) > 0 && b64s[:5] != "data:" {
		b64s = "data:image/png;base64," + b64s
	}

	// 获取验证码答案（仅用于本地/测试环境）
	var answerForClient string
	cfg := config.GetConfig()
	if cfg != nil && (cfg.AppEnv == "local" || cfg.AppEnv == "test") {
		answerForClient = answer
	}

	return &Item{
		Id:     captchaID,
		B64s:   b64s,
		Answer: answerForClient,
	}, nil
}

// Verify 校验验证码
func Verify(id, value string) bool {
	return verifyAndConsumeCaptchaAnswer(id, value)
}

// equalIgnoreCase 不区分大小写比较字符串
func equalIgnoreCase(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1 := s1[i]
		c2 := s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 32 // 转小写
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 32 // 转小写
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

package captcha

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mojocn/base64Captcha"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
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

// captchaInstance 验证码实例
var captchaInstance *base64Captcha.Captcha
var captchaOnce sync.Once

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
	// captchaLength 验证码长度
	captchaLength = 4
	// captchaCharset 验证码字符集：使用库提供的字符集，避免乱码
	// 组合字母和数字，排除容易混淆的字符（如 0/O, 1/l/I）
	captchaCharset = base64Captcha.TxtAlphabet + base64Captcha.TxtNumbers
)

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
	// 初始化验证码实例
	initCaptcha()

	// 生成唯一的验证码 ID（我们使用 UUID）
	captchaID := uuid.New().String()

	// 生成验证码（返回内部 ID、base64 编码的图片、答案和可能的错误）
	internalID, b64s, answer, err := captchaInstance.Generate()
	if err != nil {
		return nil, err
	}

	// 存储验证码答案（使用我们的 UUID 作为 key，存储实际的验证码文本）
	setCaptchaAnswer(captchaID, answer)

	// 同时存储内部 ID 到 UUID 的映射，以便后续验证时能找到
	setCaptchaAnswer("internal:"+captchaID, internalID)

	// 添加 data URI 前缀（base64Captcha 已经返回了 base64 字符串）
	if len(b64s) > 0 && b64s[:5] != "data:" {
		b64s = "data:image/png;base64," + b64s
	}

	// 获取验证码答案（仅用于本地/测试环境）
	var answerForClient string
	if config.Config.AppEnv == "local" || config.Config.AppEnv == "test" {
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
	// 初始化验证码实例
	initCaptcha()

	// 获取存储的内部验证码 ID
	internalID, ok := getCaptchaAnswer("internal:" + id)
	if !ok {
		// 如果找不到内部 ID，尝试从存储中获取答案进行直接验证
		answer, ok := getCaptchaAnswer(id)
		if !ok {
			return false
		}
		// 比较验证码（不区分大小写）
		if !equalIgnoreCase(answer, value) {
			return false
		}
		// 验证成功后删除
		deleteCaptchaAnswer(id)
		return true
	}

	// 使用 base64Captcha 的验证方法
	// 第三个参数 true 表示验证后删除
	if captchaInstance.Verify(internalID, value, true) {
		// 验证成功后删除我们的存储
		deleteCaptchaAnswer(id)
		deleteCaptchaAnswer("internal:" + id)
		return true
	}

	return false
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

package captcha

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestMemoryStoreVerifyAndDeleteIsCaseInsensitiveAndOneTime(t *testing.T) {
	store := &memoryStore{data: make(map[string]string)}
	store.Set("captcha-id", "Ab9Z", time.Minute)
	if !store.VerifyAndDelete("captcha-id", "aB9z") {
		t.Fatal("expected matching captcha to be accepted")
	}
	if store.VerifyAndDelete("captcha-id", "Ab9Z") {
		t.Fatal("consumed captcha must not be accepted twice")
	}
}

func TestMemoryStoreVerifyAndDeleteAllowsOnlyOneConcurrentSuccess(t *testing.T) {
	store := &memoryStore{data: make(map[string]string)}
	store.Set("captcha-id", "K7pQ", time.Minute)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if store.VerifyAndDelete("captcha-id", "k7Pq") {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("expected one successful captcha consumption, got %d", got)
	}
}

func TestSetCaptchaAnswerInRedisReturnsWriteFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:0",
		DialTimeout: time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	if err := setCaptchaAnswerInRedis(client, "captcha-id", "Ab9Z"); err == nil {
		t.Fatal("expected redis write failure")
	}
}

func TestGenerateReturnsCaptchaStoreFailure(t *testing.T) {
	storeErr := errors.New("redis set failed")
	item, err := generateWithStore(func(string, string) error { return storeErr })
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected captcha store failure, got %v", err)
	}
	if item != nil {
		t.Fatalf("captcha with unstored answer must not be returned: %#v", item)
	}
}

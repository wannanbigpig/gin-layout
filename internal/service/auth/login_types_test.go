package auth

import (
	"sync"
	"testing"
	"time"
)

func TestRefreshTokenLockReusesMutexBeforeExpiry(t *testing.T) {
	locker := newRefreshTokenLock(50*time.Millisecond, 10*time.Millisecond)

	first := locker.getLock("same-key")
	second := locker.getLock("same-key")

	if first != second {
		t.Fatal("expected the same mutex instance before expiry")
	}
}

func TestRefreshTokenLockCreatesNewMutexAfterExpiry(t *testing.T) {
	locker := newRefreshTokenLock(20*time.Millisecond, 5*time.Millisecond)

	first := locker.getLock("expired-key")
	time.Sleep(60 * time.Millisecond)
	second := locker.getLock("expired-key")

	if first == second {
		t.Fatal("expected a new mutex instance after expiry")
	}
}

func TestRefreshTokenLockIsSafeUnderConcurrentAccess(t *testing.T) {
	locker := newRefreshTokenLock(100*time.Millisecond, 20*time.Millisecond)

	const workers = 16
	results := make(chan *sync.Mutex, workers)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- locker.getLock("concurrent-key")
		}()
	}

	wg.Wait()
	close(results)

	var first *sync.Mutex
	for lock := range results {
		if first == nil {
			first = lock
			continue
		}
		if first != lock {
			t.Fatal("expected all goroutines to receive the same mutex instance")
		}
	}
}

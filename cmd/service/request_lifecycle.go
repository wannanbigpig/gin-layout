package service

import (
	"net/http"
	"sync"
	"time"
)

// httpRequestLifecycle 跟踪已经进入业务 handler 的请求，确保停服时业务和审计收尾
// 先于队列与数据库关闭。它使用显式计数，避免 WaitGroup 在 Wait 期间 Add 的竞态。
type httpRequestLifecycle struct {
	mu       sync.Mutex
	active   int
	draining bool
	zero     chan struct{}
}

func newHTTPRequestLifecycle() *httpRequestLifecycle {
	zero := make(chan struct{})
	close(zero)
	return &httpRequestLifecycle{zero: zero}
}

func (l *httpRequestLifecycle) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.begin() {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		defer l.done()
		next.ServeHTTP(w, r)
	})
}

func (l *httpRequestLifecycle) begin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.draining {
		return false
	}
	if l.active == 0 {
		l.zero = make(chan struct{})
	}
	l.active++
	return true
}

func (l *httpRequestLifecycle) done() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active == 0 {
		return
	}
	l.active--
	if l.active == 0 {
		close(l.zero)
	}
}

// drainAndWait 禁止新请求进入，并在 timeout 内等待现有 handler 完成。
func (l *httpRequestLifecycle) drainAndWait(timeout time.Duration) (bool, int) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	l.draining = true
	active, zero := l.active, l.zero
	l.mu.Unlock()
	if active == 0 {
		return true, 0
	}

	if timeout <= 0 {
		<-zero
		return true, 0
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-zero:
		return true, 0
	case <-timer.C:
		l.mu.Lock()
		remaining := l.active
		l.mu.Unlock()
		return false, remaining
	}
}

package middleware

import (
	"sync"
	"time"
)

var websocketAudits sync.WaitGroup

// BeginWebSocketAudit 在 Hijack 前登记请求，完成函数必须在断连审计入队后调用。
func BeginWebSocketAudit() func() {
	websocketAudits.Add(1)
	var once sync.Once
	return func() { once.Do(websocketAudits.Done) }
}

// WaitWebSocketAudits 仅在 HTTP Shutdown 完成且连接已关闭后调用，等待断连审计入队。
func WaitWebSocketAudits(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		websocketAudits.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

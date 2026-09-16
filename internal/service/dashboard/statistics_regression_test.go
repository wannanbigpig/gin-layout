package dashboard

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestErrorCategoryTotalsIncludeCodesOutsideTopTen(t *testing.T) {
	db := newDashboardTestDB(t)
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	httpCodes, bizCodes := map[string]int64{}, map[string]int64{}
	for i := 0; i < 12; i++ {
		httpCodes[strconv.Itoa(400+i)] = 1
		httpCodes[strconv.Itoa(500+i)] = 2
		bizCodes[strconv.Itoa(1000+i)] = 3
	}
	h, _ := json.Marshal(httpCodes)
	b, _ := json.Marshal(bizCodes)
	insertDailyRequestStatFull(t, db, now, map[string]any{"http_status_distribution": string(h), "biz_status_distribution": string(b), "error_requests": int64(72)})
	got := buildErrorDistribution(db, now, now.Add(24*time.Hour))
	if len(got.StatusCode) != 10 || len(got.BizStatusCodes) != 10 {
		t.Fatalf("expected Top10 display: %+v", got)
	}
	if got.Categories[0].Count != 12 || got.Categories[1].Count != 24 || got.Categories[2].Count != 36 || got.Total != 72 {
		t.Fatalf("incomplete category totals: %+v", got)
	}
}

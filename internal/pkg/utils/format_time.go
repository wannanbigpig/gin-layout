package utils

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// FormatDate 为时间字段提供统一的 JSON/SQL 编解码行为。
type FormatDate struct {
	time.Time
}

const (
	timeFormat = "2006-01-02 15:04:05"
)

// ParseTimeBoundary 解析列表筛选的时间范围边界（start_time/end_time）。
// 前端发送带时区的 ISO-8601 绝对时刻（如 2026-05-31T16:00:00.000Z），统一转换到服务器本地时区，
// 与按 time.Local 落库的 created_at（DATETIME 墙钟）对齐，避免浏览器时区≠服务器时区时整段区间偏移。
// 兼容历史无时区墙钟串（按服务器本地解析）。返回的 bool 表示是否解析出有效边界。
func ParseTimeBoundary(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.In(time.Local), true
	}
	if t, err := time.ParseInLocation(timeFormat, value, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// MarshalJSON 以固定格式输出 JSON 时间字符串。
func (t FormatDate) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Format(timeFormat))), nil
}

// Value 实现 driver.Valuer 接口。
func (t FormatDate) Value() (driver.Value, error) {
	var zeroTime time.Time
	if t.Time.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t.Time, nil
}

// Scan 实现 sql.Scanner 接口。
func (t *FormatDate) Scan(v interface{}) error {
	if value, ok := v.(time.Time); ok {
		*t = FormatDate{value}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}

// String 返回可读的时间字符串。
func (t *FormatDate) String() string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Time.Format(timeFormat)
}

// UnmarshalJSON 解析固定格式的 JSON 时间字符串。
func (t *FormatDate) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == "null" {
		return nil
	}
	t1, err := time.ParseInLocation(timeFormat, strings.Trim(str, "\""), time.Local)
	*t = FormatDate{t1}
	return err
}

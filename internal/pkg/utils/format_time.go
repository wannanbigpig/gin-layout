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

// MarshalJSON 以固定格式输出 JSON 时间字符串。
func (t FormatDate) MarshalJSON() ([]byte, error) {
	if &t == nil || t.IsZero() {
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
	return fmt.Sprintf("%s", t.Time.Format(timeFormat))
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

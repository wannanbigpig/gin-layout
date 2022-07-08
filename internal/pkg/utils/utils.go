package utils

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

type FormatDate struct {
	time.Time
}

func (t FormatDate) MarshalJSON() ([]byte, error) {
	if &t == nil || t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Format("2006-01-02 15:04:05"))), nil
}

func (t FormatDate) Value() (driver.Value, error) {
	var zeroTime time.Time
	if t.Time.UnixNano() == zeroTime.UnixNano() {
		return nil, nil
	}
	return t.Time, nil
}

func (t *FormatDate) Scan(v interface{}) error {
	if value, ok := v.(time.Time); ok {
		*t = FormatDate{value}
		return nil
	}
	return fmt.Errorf("can not convert %v to timestamp", v)
}

func (t *FormatDate) String() string {
	if t == nil || t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%s", t.Time.Format("2006-01-02 15:04:05"))
}

func (t *FormatDate) UnmarshalJSON(data []byte) error {
	str := string(data)
	if str == "null" {
		return nil
	}
	t1, err := time.ParseInLocation("2006-01-02 15:04:05", strings.Trim(str, "\""), time.Local)
	*t = FormatDate{t1}
	return err
}

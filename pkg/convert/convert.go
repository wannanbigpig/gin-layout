package convert

import "time"

func GetString(val interface{}) (s string) {
	s, _ = val.(string)
	return
}

// GetBool returns the value associated with the key as a boolean.
func GetBool(val interface{}) (b bool) {
	b, _ = val.(bool)
	return
}

// GetInt returns the value associated with the key as an integer.
func GetInt(val interface{}) (i int) {
	i, _ = val.(int)
	return
}

// GetInt64 returns the value associated with the key as an integer.
func GetInt64(val interface{}) (i64 int64) {
	i64, _ = val.(int64)
	return
}

// GetUint returns the value associated with the key as an unsigned integer.
func GetUint(val interface{}) (ui uint) {
	ui, _ = val.(uint)
	return
}

// GetUint64 returns the value associated with the key as an unsigned integer.
func GetUint64(val interface{}) (ui64 uint64) {
	ui64, _ = val.(uint64)
	return
}

// GetFloat64 returns the value associated with the key as a float64.
func GetFloat64(val interface{}) (f64 float64) {
	f64, _ = val.(float64)
	return
}

// GetTime returns the value associated with the key as time.
func GetTime(val interface{}) (t time.Time) {
	t, _ = val.(time.Time)
	return
}

// GetDuration returns the value associated with the key as a duration.
func GetDuration(val interface{}) (d time.Duration) {
	d, _ = val.(time.Duration)
	return
}

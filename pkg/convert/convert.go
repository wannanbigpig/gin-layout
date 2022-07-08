package convert

import "time"

func GetString(val interface{}) (s string, ok bool) {
	s, ok = val.(string)
	return
}

// GetBool returns the value associated with the key as a boolean.
func GetBool(val interface{}) (b bool, ok bool) {
	b, ok = val.(bool)
	return
}

// GetInt returns the value associated with the key as an integer.
func GetInt(val interface{}) (i int, ok bool) {
	i, ok = val.(int)
	return
}

// GetInt64 returns the value associated with the key as an integer.
func GetInt64(val interface{}) (i64 int64, ok bool) {
	i64, ok = val.(int64)
	return
}

// GetUint returns the value associated with the key as an unsigned integer.
func GetUint(val interface{}) (ui uint, ok bool) {
	ui, ok = val.(uint)
	return
}

// GetUint64 returns the value associated with the key as an unsigned integer.
func GetUint64(val interface{}) (ui64 uint64, ok bool) {
	ui64, ok = val.(uint64)
	return
}

// GetFloat64 returns the value associated with the key as a float64.
func GetFloat64(val interface{}) (f64 float64, ok bool) {
	f64, ok = val.(float64)
	return
}

// GetTime returns the value associated with the key as time.
func GetTime(val interface{}) (t time.Time, ok bool) {
	t, ok = val.(time.Time)
	return
}

// GetDuration returns the value associated with the key as a duration.
func GetDuration(val interface{}) (d time.Duration, ok bool) {
	d, ok = val.(time.Duration)
	return
}

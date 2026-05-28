package errors

// enUSText 汇总各模块英文错误码文案。
// 模块拆分位于 en-us_common.go / en-us_file.go / en-us_auth.go /
// en-us_role.go / en-us_menu.go / en-us_dept.go，通过 init() 合并到此处。
var enUSText = map[int]string{}

// enUSTextKey 汇总各模块英文文案 key。
// 拆分文件通过 init() 合并到此处。
var enUSTextKey = map[string]string{}

// mergeEnUSText 将模块内英文文案合并到 enUSText。
func mergeEnUSText(src map[int]string) {
	for code, msg := range src {
		enUSText[code] = msg
	}
}

// mergeEnUSTextKey 将模块内文案 key 合并到 enUSTextKey。
func mergeEnUSTextKey(src map[string]string) {
	for key, msg := range src {
		enUSTextKey[key] = msg
	}
}

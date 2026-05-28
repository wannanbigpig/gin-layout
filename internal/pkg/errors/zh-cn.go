package errors

// zhCNText 汇总各模块的中文错误码文案。
// 各模块文案通过 zh-cn_*.go 的 init() 合并到此处。
var zhCNText = map[int]string{}

// zhCNTextKey 汇总各模块的中文文案 key。
// 拆分文件通过 init() 合并到此处。
var zhCNTextKey = map[string]string{}

// mergeZhCNText 将模块内中文文案合并到 zhCNText。
func mergeZhCNText(src map[int]string) {
	for code, msg := range src {
		zhCNText[code] = msg
	}
}

// mergeZhCNTextKey 将模块内文案 key 合并到 zhCNTextKey。
func mergeZhCNTextKey(src map[string]string) {
	for key, msg := range src {
		zhCNTextKey[key] = msg
	}
}

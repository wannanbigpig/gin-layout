package access

import (
	"fmt"
	"sync"
)

// BindingSource 表示绑定来源类型。
type BindingSource string

const (
	// BindingSourceBuiltin 内置默认绑定。
	BindingSourceBuiltin BindingSource = "builtin"
	// BindingSourceExternal 外部配置绑定（来自配置文件或代码生成器）。
	BindingSourceExternal BindingSource = "external"
)

// ExternalMenuAPIBinding 外部菜单接口绑定配置。
type ExternalMenuAPIBinding struct {
	// MenuCode 菜单编码。
	MenuCode string
	// Route 绑定的接口路由。
	Route string
	// Method 绑定的 HTTP 方法。
	Method string
	// Source 绑定来源（builtin/external）。
	Source BindingSource
	// Description 绑定描述（可选，用于文档）。
	Description string
}

// externalBindings 全局外部绑定存储。
var externalBindings []ExternalMenuAPIBinding

// externalBindingsMu 外部绑定读写锁。
var externalBindingsMu sync.RWMutex

// AddExternalBinding 添加外部菜单接口绑定（供代码生成器或配置加载调用）。
func AddExternalBinding(binding ExternalMenuAPIBinding) {
	externalBindingsMu.Lock()
	defer externalBindingsMu.Unlock()

	if binding.Source == "" {
		binding.Source = BindingSourceExternal
	}
	externalBindings = append(externalBindings, binding)
}

// AddExternalBindings 批量添加外部菜单接口绑定。
func AddExternalBindings(bindings []ExternalMenuAPIBinding) {
	externalBindingsMu.Lock()
	defer externalBindingsMu.Unlock()

	for i := range bindings {
		if bindings[i].Source == "" {
			bindings[i].Source = BindingSourceExternal
		}
	}
	externalBindings = append(externalBindings, bindings...)
}

// LoadExternalBindings 加载所有外部绑定配置。
func LoadExternalBindings() []ExternalMenuAPIBinding {
	externalBindingsMu.RLock()
	defer externalBindingsMu.RUnlock()

	if len(externalBindings) == 0 {
		return nil
	}
	cloned := make([]ExternalMenuAPIBinding, len(externalBindings))
	copy(cloned, externalBindings)
	return cloned
}

// ClearExternalBindings 清除所有外部绑定（主要用于测试）。
func ClearExternalBindings() {
	externalBindingsMu.Lock()
	defer externalBindingsMu.Unlock()
	externalBindings = nil
}

// MergeBindings 合并内置绑定和外部绑定，去重后返回。
func MergeBindings(builtin []defaultMenuAPIBinding, external []ExternalMenuAPIBinding) []defaultMenuAPIBinding {
	if len(external) == 0 {
		return cloneMenuAPIBindings(builtin)
	}
	if len(builtin) == 0 {
		result := make([]defaultMenuAPIBinding, len(external))
		for i, b := range external {
			result[i] = defaultMenuAPIBinding{
				MenuCode: b.MenuCode,
				Route:    b.Route,
				Method:   b.Method,
			}
		}
		return result
	}

	// 使用 map 去重，key 为 method:route:menuCode
	seen := make(map[string]struct{}, len(builtin)+len(external))
	result := make([]defaultMenuAPIBinding, 0, len(builtin)+len(external))

	// 先添加内置绑定
	for _, b := range builtin {
		key := b.Method + ":" + b.Route + ":" + b.MenuCode
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, b)
		}
	}

	// 再添加外部绑定（去重）
	for _, b := range external {
		key := b.Method + ":" + b.Route + ":" + b.MenuCode
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, defaultMenuAPIBinding{
				MenuCode: b.MenuCode,
				Route:    b.Route,
				Method:   b.Method,
			})
		}
	}

	return result
}

// GetBindingStats 返回绑定统计信息。
func GetBindingStats(builtin []defaultMenuAPIBinding, external []ExternalMenuAPIBinding) map[string]int {
	stats := map[string]int{
		"builtin_count":  len(builtin),
		"external_count": len(external),
	}

	// 计算去重后的总数
	merged := MergeBindings(builtin, external)
	stats["total_count"] = len(merged)
	stats["duplicate_count"] = len(builtin) + len(external) - len(merged)

	return stats
}

// BindingDiff 返回外部绑定与内置绑定的差异（新增的绑定）。
func BindingDiff(builtin []defaultMenuAPIBinding, external []ExternalMenuAPIBinding) []ExternalMenuAPIBinding {
	if len(external) == 0 {
		return nil
	}

	builtinSet := make(map[string]struct{}, len(builtin))
	for _, b := range builtin {
		key := b.Method + ":" + b.Route + ":" + b.MenuCode
		builtinSet[key] = struct{}{}
	}

	var diff []ExternalMenuAPIBinding
	for _, b := range external {
		key := b.Method + ":" + b.Route + ":" + b.MenuCode
		if _, exists := builtinSet[key]; !exists {
			diff = append(diff, b)
		}
	}

	return diff
}

// FormatExternalBindings 格式化外部绑定为 Go 代码片段（用于生成器输出）。
func FormatExternalBindings(bindings []ExternalMenuAPIBinding) string {
	if len(bindings) == 0 {
		return ""
	}

	code := "var externalMenuAPIBindings = [...]ExternalMenuAPIBinding{\n"
	for _, b := range bindings {
		desc := b.Description
		if desc != "" {
			code += fmt.Sprintf("\t// %s\n", desc)
		}
		code += fmt.Sprintf("\t{MenuCode: %q, Route: %q, Method: %q, Source: %q},\n",
			b.MenuCode, b.Route, b.Method, b.Source)
	}
	code += "}"
	return code
}

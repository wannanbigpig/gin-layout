// Package openapi 提供基于路由树自动生成 OpenAPI 3.0 文档的命令行工具。
package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/routers"
)

const (
	defaultOutputPath = "docs/openapi.yaml"
	docTitle          = "go-layout API"
	docDescription    = "由 `go-layout command gen-openapi` 基于声明式路由树自动生成。"
)

var (
	outputPath string

	// Cmd 暴露给上层注册的子命令。
	Cmd = &cobra.Command{
		Use:   "gen-openapi",
		Short: "Generate OpenAPI 3.0 documentation from route tree",
		Long:  "Walks the AdminRouteTree and emits an OpenAPI 3.0 YAML document containing path, method, summary, tags and security metadata.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenOpenAPI()
		},
	}
)

func init() {
	Cmd.Flags().StringVarP(&outputPath, "output", "o", defaultOutputPath, "Output file path for the generated OpenAPI document")
}

// endpoint 表示路由树中的一个 API 端点（仅文档生成所需的元数据）。
type endpoint struct {
	Path     string                 // 完整路径，含 admin/v1 等前缀
	Method   string                 // HTTP 方法（小写）
	Summary  string                 // 路由标题
	Desc     string                 // 路由描述
	Tag      string                 // 路由分组标签
	Auth     routers.AuthMode       // 兼容的 0/1/2 认证模式
	Security routers.SecurityPolicy // 实际凭证与权限策略
}

// runGenOpenAPI 执行 OpenAPI 文档生成。
func runGenOpenAPI() error {
	catalog, err := routers.CompileRouteTree(routers.AppRouteTreeWithOptions(routers.BuildOptions{Debug: false}))
	if err != nil {
		return fmt.Errorf("compile AppRouteTree: %w", err)
	}
	endpoints := collectEndpoints(catalog)

	if len(endpoints) == 0 {
		return fmt.Errorf("no routes collected from AppRouteTree")
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	yaml := buildYAML(endpoints)

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(abs, []byte(yaml), 0o644); err != nil {
		return fmt.Errorf("write openapi file: %w", err)
	}

	fmt.Printf("OpenAPI document generated: %s (%d endpoints)\n", abs, len(endpoints))
	return nil
}

func collectEndpoints(catalog *routers.Catalog) []endpoint {
	routes := catalog.Routes()
	endpoints := make([]endpoint, 0, len(routes))
	for _, route := range routes {
		endpoints = append(endpoints, endpoint{
			Path: route.Path, Method: strings.ToLower(route.Method), Summary: route.Title,
			Desc: strings.TrimSpace(route.Desc), Tag: resolveTag(route.GroupCode, route.Path),
			Auth: route.Auth, Security: route.Security,
		})
	}
	return endpoints
}

// resolveTag 在分组未声明 GroupCode 时，回退到路径段。
func resolveTag(tag, fullPath string) string {
	if tag != "" {
		return tag
	}
	for _, seg := range strings.Split(strings.Trim(fullPath, "/"), "/") {
		if seg == "" || seg == "admin" || strings.HasPrefix(seg, "v") {
			continue
		}
		return seg
	}
	return "default"
}

// buildYAML 根据端点列表构造 OpenAPI 3.0 YAML 字符串。
// 自行序列化以保证字段顺序稳定，避免依赖 yaml 包的 map 迭代顺序。
func buildYAML(endpoints []endpoint) string {
	var b strings.Builder

	b.WriteString("openapi: 3.0.0\n")
	b.WriteString("info:\n")
	b.WriteString("  title: ")
	b.WriteString(quote(docTitle))
	b.WriteString("\n")
	b.WriteString("  version: ")
	b.WriteString(quote(global.Version))
	b.WriteString("\n")
	b.WriteString("  description: ")
	b.WriteString(quote(docDescription))
	b.WriteString("\n")

	// tags
	tagSet := map[string]struct{}{}
	for _, ep := range endpoints {
		tagSet[ep.Tag] = struct{}{}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	if len(tags) > 0 {
		b.WriteString("tags:\n")
		for _, t := range tags {
			b.WriteString("  - name: ")
			b.WriteString(quote(t))
			b.WriteString("\n")
		}
	}

	writeComponents(&b)

	// paths
	b.WriteString("paths:\n")

	grouped := map[string][]endpoint{}
	pathOrder := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if _, ok := grouped[ep.Path]; !ok {
			pathOrder = append(pathOrder, ep.Path)
		}
		grouped[ep.Path] = append(grouped[ep.Path], ep)
	}
	sort.Strings(pathOrder)

	for _, p := range pathOrder {
		b.WriteString("  ")
		b.WriteString(quote(toOpenAPIPath(p)))
		b.WriteString(":\n")

		methods := grouped[p]
		sort.Slice(methods, func(i, j int) bool { return methods[i].Method < methods[j].Method })

		for _, ep := range methods {
			writeOperation(&b, ep, p)
		}
	}

	return b.String()
}

// writeComponents 写入 components 段：安全方案与统一响应 schema。
func writeComponents(b *strings.Builder) {
	b.WriteString("components:\n")
	b.WriteString("  securitySchemes:\n")
	b.WriteString("    BearerAuth:\n")
	b.WriteString("      type: http\n")
	b.WriteString("      scheme: bearer\n")
	b.WriteString("      bearerFormat: JWT\n")
	b.WriteString("      description: ")
	b.WriteString(quote("通过 Authorization: Bearer <token> 携带访问令牌"))
	b.WriteString("\n")
	b.WriteString("    RefreshCookie:\n")
	b.WriteString("      type: apiKey\n")
	b.WriteString("      in: cookie\n")
	b.WriteString("      name: refresh_token\n")
	b.WriteString("      description: ")
	b.WriteString(quote("通过 HttpOnly refresh_token Cookie 完成会话刷新或退出"))
	b.WriteString("\n")
	b.WriteString("    WebSocketTicket:\n")
	b.WriteString("      type: apiKey\n")
	b.WriteString("      in: query\n")
	b.WriteString("      name: ticket\n")
	b.WriteString("      description: ")
	b.WriteString(quote("通过一次性 ticket 建立 WebSocket 连接"))
	b.WriteString("\n")
	b.WriteString("  schemas:\n")
	b.WriteString("    Response:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: ")
	b.WriteString(quote("统一响应包络"))
	b.WriteString("\n")
	b.WriteString("      required:\n")
	b.WriteString("        - code\n")
	b.WriteString("        - msg\n")
	b.WriteString("        - data\n")
	b.WriteString("        - cost\n")
	b.WriteString("        - request_id\n")
	b.WriteString("      properties:\n")
	b.WriteString("        code:\n")
	b.WriteString("          type: integer\n")
	b.WriteString("          description: ")
	b.WriteString(quote("业务状态码（0 表示成功）"))
	b.WriteString("\n")
	b.WriteString("          example: 0\n")
	b.WriteString("        msg:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: ")
	b.WriteString(quote("提示信息"))
	b.WriteString("\n")
	b.WriteString("          example: ")
	b.WriteString(quote("success"))
	b.WriteString("\n")
	b.WriteString("        data:\n")
	b.WriteString("          description: ")
	b.WriteString(quote("业务数据载荷"))
	b.WriteString("\n")
	b.WriteString("          nullable: true\n")
	b.WriteString("        cost:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: ")
	b.WriteString(quote("请求处理耗时"))
	b.WriteString("\n")
	b.WriteString("          example: ")
	b.WriteString(quote("3.2ms"))
	b.WriteString("\n")
	b.WriteString("        request_id:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: ")
	b.WriteString(quote("请求唯一标识"))
	b.WriteString("\n")
	b.WriteString("          example: ")
	b.WriteString(quote("req_20260423_0001"))
	b.WriteString("\n")
}

// writeOperation 写入单个 method 下的操作详情。
func writeOperation(b *strings.Builder, ep endpoint, originalPath string) {
	indent := "    "

	b.WriteString(indent)
	b.WriteString(ep.Method)
	b.WriteString(":\n")

	b.WriteString(indent)
	b.WriteString("  tags:\n")
	b.WriteString(indent)
	b.WriteString("    - ")
	b.WriteString(quote(ep.Tag))
	b.WriteString("\n")

	b.WriteString(indent)
	b.WriteString("  summary: ")
	b.WriteString(quote(ep.Summary))
	b.WriteString("\n")

	desc := ep.Desc
	if desc == "" {
		desc = ep.Summary
	}
	b.WriteString(indent)
	b.WriteString("  description: ")
	b.WriteString(quote(desc))
	b.WriteString("\n")

	b.WriteString(indent)
	b.WriteString("  operationId: ")
	b.WriteString(quote(operationID(ep.Method, originalPath)))
	b.WriteString("\n")

	pathParams := extractPathParams(originalPath)
	if len(pathParams) > 0 {
		b.WriteString(indent)
		b.WriteString("  parameters:\n")
		for _, name := range pathParams {
			b.WriteString(indent)
			b.WriteString("    - name: ")
			b.WriteString(quote(name))
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString("      in: path\n")
			b.WriteString(indent)
			b.WriteString("      required: true\n")
			b.WriteString(indent)
			b.WriteString("      description: ")
			b.WriteString(quote("路径参数 " + name))
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString("      schema:\n")
			b.WriteString(indent)
			b.WriteString("        type: string\n")
		}
	}

	switch ep.Security {
	case routers.SecurityAccessToken, routers.SecurityAccessTokenPermission:
		b.WriteString(indent)
		b.WriteString("  security:\n")
		b.WriteString(indent)
		b.WriteString("    - BearerAuth: []\n")
	case routers.SecurityRefreshCookie:
		b.WriteString(indent)
		b.WriteString("  security:\n")
		b.WriteString(indent)
		b.WriteString("    - RefreshCookie: []\n")
	case routers.SecurityWebSocketTicket:
		b.WriteString(indent)
		b.WriteString("  security:\n")
		b.WriteString(indent)
		b.WriteString("    - WebSocketTicket: []\n")
	default:
		b.WriteString(indent)
		b.WriteString("  security: []\n")
	}

	b.WriteString(indent)
	b.WriteString("  x-auth-mode: ")
	b.WriteString(quote(authLabel(ep.Auth)))
	b.WriteString("\n")
	b.WriteString(indent)
	b.WriteString("  x-security-policy: ")
	b.WriteString(quote(ep.Security.String()))
	b.WriteString("\n")

	b.WriteString(indent)
	b.WriteString("  responses:\n")
	b.WriteString(indent)
	b.WriteString("    '200':\n")
	b.WriteString(indent)
	b.WriteString("      description: OK\n")
	b.WriteString(indent)
	b.WriteString("      content:\n")
	b.WriteString(indent)
	b.WriteString("        application/json:\n")
	b.WriteString(indent)
	b.WriteString("          schema:\n")
	b.WriteString(indent)
	b.WriteString("            $ref: '#/components/schemas/Response'\n")
}

// toOpenAPIPath 将 gin 风格 :name 转换为 OpenAPI 风格 {name}。
func toOpenAPIPath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") {
			parts[i] = "{" + seg[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

// extractPathParams 提取 :name 形式的路径参数名。
func extractPathParams(p string) []string {
	var params []string
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ":") && len(seg) > 1 {
			params = append(params, seg[1:])
		}
	}
	return params
}

// operationID 生成稳定的 operationId。
func operationID(method, path string) string {
	cleaned := strings.Trim(path, "/")
	cleaned = strings.ReplaceAll(cleaned, "/", "_")
	cleaned = strings.ReplaceAll(cleaned, ":", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	if cleaned == "" {
		cleaned = "root"
	}
	return strings.ToLower(method) + "_" + cleaned
}

// authLabel 将认证模式转为 OpenAPI 扩展字段中的可读标签。
func authLabel(m routers.AuthMode) string {
	switch m {
	case routers.AuthModeNone:
		return "none"
	case routers.AuthModeLogin:
		return "login"
	case routers.AuthModeAuth:
		return "auth"
	default:
		return "unknown"
	}
}

// quote 以双引号字符串形式输出 YAML 标量，转义需要的特殊字符。
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

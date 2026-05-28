package genErrorCodes

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// 默认输入与输出路径，可通过 flag 覆盖。
const (
	defaultErrorsDir = "internal/pkg/errors"
	defaultJSONOut   = "docs/error_codes.json"
	defaultTSOut     = "docs/error_codes.ts"
)

var (
	errorsDir string
	jsonOut   string
	tsOut     string

	// Cmd 扫描错误码常量并生成 JSON / TypeScript 文件。
	Cmd = &cobra.Command{
		Use:     "gen-error-codes",
		Short:   "Generate frontend-friendly error code files (JSON & TypeScript) from internal/pkg/errors.",
		Long:    "Scan the Go error code constants under internal/pkg/errors/code_*.go and emit docs/error_codes.json and docs/error_codes.ts so frontend projects can share the same numeric codes.",
		Example: "go-layout command gen-error-codes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
	}
)

func init() {
	Cmd.Flags().StringVar(&errorsDir, "src", defaultErrorsDir, "Directory containing error code files (code_*.go)")
	Cmd.Flags().StringVar(&jsonOut, "json", defaultJSONOut, "Output path of the generated JSON file")
	Cmd.Flags().StringVar(&tsOut, "ts", defaultTSOut, "Output path of the generated TypeScript file")
}

// errorCode 单个错误码项。
type errorCode struct {
	Name    string `json:"name"`
	Code    int    `json:"code"`
	Comment string `json:"comment,omitempty"`
	Source  string `json:"source"`
}

// fileGroup 单个 code_*.go 文件中的错误码集合。
type fileGroup struct {
	File    string      `json:"file"`
	Comment string      `json:"comment,omitempty"`
	Codes   []errorCode `json:"codes"`
}

// outputDoc JSON 输出结构。
type outputDoc struct {
	Generator string      `json:"generator"`
	Source    string      `json:"source"`
	Total     int         `json:"total"`
	Groups    []fileGroup `json:"groups"`
}

func run() error {
	files, err := filepath.Glob(filepath.Join(errorsDir, "code_*.go"))
	if err != nil {
		return fmt.Errorf("glob error code files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no code_*.go files found in %s", errorsDir)
	}
	sort.Strings(files)

	var (
		groups []fileGroup
		all    []errorCode
		seen   = make(map[string]string)
	)

	fset := token.NewFileSet()
	for _, file := range files {
		// 跳过测试文件。
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		group, err := parseFile(fset, file)
		if err != nil {
			return err
		}
		if len(group.Codes) == 0 {
			continue
		}
		// 同名常量校验，避免出现重复。
		for _, c := range group.Codes {
			if origin, ok := seen[c.Name]; ok {
				return fmt.Errorf("duplicate error code constant %q in %s and %s", c.Name, origin, file)
			}
			seen[c.Name] = file
		}
		groups = append(groups, group)
		all = append(all, group.Codes...)
	}

	// 按 code 升序汇总，方便阅读。
	sort.SliceStable(all, func(i, j int) bool { return all[i].Code < all[j].Code })

	doc := outputDoc{
		Generator: "go-layout command gen-error-codes",
		Source:    errorsDir,
		Total:     len(all),
		Groups:    groups,
	}

	if err := writeJSON(jsonOut, doc); err != nil {
		return err
	}
	if err := writeTS(tsOut, all); err != nil {
		return err
	}

	fmt.Printf("Generated %d error codes from %d files.\n", len(all), len(groups))
	fmt.Printf("  - %s\n", jsonOut)
	fmt.Printf("  - %s\n", tsOut)
	return nil
}

// parseFile 解析单个 code_*.go 文件，提取所有 int 类型常量。
func parseFile(fset *token.FileSet, path string) (fileGroup, error) {
	group := fileGroup{File: filepath.ToSlash(path)}

	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return group, fmt.Errorf("parse %s: %w", path, err)
	}

	// 文件级 doc 注释作为 group 注释（去掉 package 行注释，仅取文件顶部 // 注释段）。
	if f.Doc != nil {
		group.Comment = trimComment(f.Doc.Text())
	}

	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		// const 块上方的注释（如 // 通用错误码（0-10999）。）
		blockComment := ""
		if gen.Doc != nil {
			blockComment = trimComment(gen.Doc.Text())
		}
		if group.Comment == "" {
			group.Comment = blockComment
		}

		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			// 忽略带显式类型且非数字字面量的常量（如字符串常量）。
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.INT {
					continue
				}
				code, err := strconv.Atoi(lit.Value)
				if err != nil {
					continue
				}
				comment := ""
				if vs.Doc != nil {
					comment = trimComment(vs.Doc.Text())
				} else if vs.Comment != nil {
					comment = trimComment(vs.Comment.Text())
				}
				group.Codes = append(group.Codes, errorCode{
					Name:    name.Name,
					Code:    code,
					Comment: comment,
					Source:  filepath.ToSlash(path),
				})
			}
		}
	}

	// 文件内同样按 code 升序。
	sort.SliceStable(group.Codes, func(i, j int) bool { return group.Codes[i].Code < group.Codes[j].Code })
	return group, nil
}

func trimComment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// 去除单行末尾的句号变体。
	return strings.TrimRight(s, "\n")
}

func writeJSON(path string, doc outputDoc) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	buf, err := json.MarshalIndent(doc, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	buf = append(buf, '\n')
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeTS(path string, codes []errorCode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	var b strings.Builder
	b.WriteString("/* eslint-disable */\n")
	b.WriteString("// Code generated by `go-layout command gen-error-codes`. DO NOT EDIT.\n")
	b.WriteString("// Source: internal/pkg/errors/code_*.go\n\n")
	b.WriteString("export const ErrorCode = {\n")

	// 按业务模块的常量名字典序输出比较稳定，但同时希望 SUCCESS=0 排前；
	// 这里按 (code asc, name asc) 排，符合直觉。
	sorted := make([]errorCode, len(codes))
	copy(sorted, codes)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Code != sorted[j].Code {
			return sorted[i].Code < sorted[j].Code
		}
		return sorted[i].Name < sorted[j].Name
	})

	for _, c := range sorted {
		if c.Comment != "" {
			// 单行展开为 // 注释，多行则裁成首行避免破坏代码块。
			line := strings.SplitN(c.Comment, "\n", 2)[0]
			line = strings.TrimSpace(line)
			if line != "" {
				fmt.Fprintf(&b, "    /** %s */\n", line)
			}
		}
		fmt.Fprintf(&b, "    %s: %d,\n", c.Name, c.Code)
	}

	b.WriteString("} as const\n\n")
	b.WriteString("export type ErrorCodeName = keyof typeof ErrorCode\n")
	b.WriteString("export type ErrorCodeType = (typeof ErrorCode)[ErrorCodeName]\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

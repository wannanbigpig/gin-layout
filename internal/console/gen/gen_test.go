package gen

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestBuildBindingTagKeepsNumericEnumConstraint(t *testing.T) {
	got := buildBindingTag(FieldSchema{
		ColumnName: "status",
		GoType:     "uint8",
		EnumValues: "0 1",
		DefaultVal: "0",
	})
	if got != "gte=0,oneof=0 1" {
		t.Fatalf("unexpected binding tag: %q", got)
	}
}

func TestExtractEnumValuesOnlyInfersBooleanTinyint(t *testing.T) {
	if got := extractEnumValues("tinyint(1)"); got != "0 1" {
		t.Fatalf("expected boolean enum, got %q", got)
	}
	if got := extractEnumValues("tinyint unsigned"); got != "" {
		t.Fatalf("must not restrict a general tinyint to 0/1, got %q", got)
	}
}

func TestTemplatesDoNotAssumeOptionalOwnershipOrNameColumns(t *testing.T) {
	data := buildTemplateData(&TableSchema{
		TableName: "article",
		Module:    "article",
		MenuTitle: "文章",
		Fields: []FieldSchema{
			{Name: "Content", ColumnName: "content", ColumnType: "json", GoType: "datatypes.JSON", GoTag: "column:content;type:json"},
			{Name: "PublishedAt", ColumnName: "published_at", ColumnType: "datetime", GoType: "utils.FormatDate", GoTag: "column:published_at;type:datetime"},
		},
	}, "example.com/project")

	modelSource := renderGeneratorTemplate(t, "model.go.tmpl", data)
	for _, expected := range []string{`"gorm.io/datatypes"`, `"example.com/project/internal/pkg/utils"`} {
		if !strings.Contains(modelSource, expected) {
			t.Fatalf("model template missing %s:\n%s", expected, modelSource)
		}
	}
	if strings.Contains(modelSource, "modelDict") {
		t.Fatalf("model template imported unused status dictionary:\n%s", modelSource)
	}

	serviceSource := renderGeneratorTemplate(t, "service.go.tmpl", data)
	for _, unexpected := range []string{".CreatedBy", ".DeptId", `"dept_id"`, `"created_by"`, "func (s *ArticleService) Options"} {
		if strings.Contains(serviceSource, unexpected) {
			t.Fatalf("service template assumed absent field %q:\n%s", unexpected, serviceSource)
		}
	}

	resourceSource := renderGeneratorTemplate(t, "resource.go.tmpl", data)
	if strings.Contains(resourceSource, "item.Name") || strings.Contains(resourceSource, "ArticleOptionResources") {
		t.Fatalf("resource template assumed absent name field:\n%s", resourceSource)
	}

	for name, source := range map[string]string{
		"model.go":      modelSource,
		"service.go":    serviceSource,
		"resource.go":   resourceSource,
		"form.go":       renderGeneratorTemplate(t, "form.go.tmpl", data),
		"controller.go": renderGeneratorTemplate(t, "controller.go.tmpl", data),
	} {
		if _, err := parser.ParseFile(token.NewFileSet(), name, source, parser.AllErrors); err != nil {
			t.Fatalf("generated %s is not valid Go: %v\n%s", name, err, source)
		}
	}
	migrationSource := renderGeneratorTemplate(t, "migration.up.sql.tmpl", struct {
		*TemplateData
		Comment string
	}{TemplateData: data, Comment: "文章"})
	if !strings.Contains(migrationSource, "`content` json") || !strings.Contains(migrationSource, "`published_at` datetime") {
		t.Fatalf("migration template lost database column metadata:\n%s", migrationSource)
	}
}

func TestFrontendListTemplateUsesNormalizedRequestContract(t *testing.T) {
	data := buildTemplateData(&TableSchema{
		TableName: "article",
		Module:    "article",
		MenuTitle: "文章",
		Fields: []FieldSchema{
			{Name: "Name", ColumnName: "name", ColumnType: "varchar(60)", GoType: "string", GoTag: "column:name", MaxLength: 60},
		},
	}, "example.com/project")

	source := renderGeneratorTemplate(t, "index.vue.tmpl", data)
	if !strings.Contains(source, "tableData.value = res.list") || !strings.Contains(source, "total.value = res.total") {
		t.Fatalf("frontend template does not use normalized pagination contract:\n%s", source)
	}
	if strings.Contains(source, "res.data.data") || strings.Contains(source, "res.data.total") {
		t.Fatalf("frontend template still expects Axios response nesting:\n%s", source)
	}

	apiSource := renderGeneratorTemplate(t, "api.ts.tmpl", data)
	if !strings.Contains(apiSource, "import { get, post } from '@/utils/request'") {
		t.Fatalf("frontend API template does not use normalized request helpers:\n%s", apiSource)
	}
	if !strings.Contains(apiSource, "return get<ArticleListResult>('/v1/article/list', { ...params })") {
		t.Fatalf("frontend API template does not return normalized list data:\n%s", apiSource)
	}
	if strings.Contains(apiSource, "request.get") || strings.Contains(apiSource, "'/admin/v1/") {
		t.Fatalf("frontend API template still uses Axios response contract or duplicated admin prefix:\n%s", apiSource)
	}
}

func TestRouteGroupTemplateInheritsPermissionGroupAuthMode(t *testing.T) {
	data := buildTemplateData(&TableSchema{
		TableName: "article",
		Module:    "article",
		MenuTitle: "文章",
		Fields: []FieldSchema{
			{Name: "Name", ColumnName: "name", ColumnType: "varchar(60)", GoType: "string", GoTag: "column:name", MaxLength: 60},
		},
	}, "example.com/project")

	source := renderGeneratorTemplate(t, "route_group.go.tmpl", data)
	if strings.Contains(source, "AuthModeAuth") {
		t.Fatalf("route template still passes the removed per-route auth argument:\n%s", source)
	}
	if !strings.Contains(source, `GET("list", "文章列表", deps.ArticleController.List)`) {
		t.Fatalf("route template does not use the current route constructor contract:\n%s", source)
	}
}

func TestRenderMenuSQLMatchesCurrentMenuSchema(t *testing.T) {
	data := buildTemplateData(&TableSchema{
		TableName: "article",
		Module:    "article",
		MenuTitle: "文章",
		MenuPID:   10,
	}, "example.com/project")

	sql := renderMenuSQL(data)
	if strings.Contains(sql, "`title_i18n`") || strings.Contains(sql, "`is_cache`") {
		t.Fatalf("menu SQL contains removed menu columns:\n%s", sql)
	}
	if !strings.Contains(sql, "INSERT INTO `menu_i18n`") || !strings.Contains(sql, "`component_key`") {
		t.Fatalf("menu SQL does not write current menu and i18n tables:\n%s", sql)
	}
}

func TestDryRunStillValidatesTemplates(t *testing.T) {
	root := t.TempDir()
	templateDir := filepath.Join(root, "internal", "console", "gen", "templates")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "broken.tmpl"), []byte("{{"), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	generator := NewGenerator(&TableSchema{}, "example.com/project", root, true, false, "")
	if err := generator.renderAndWrite("broken.tmpl", "internal/model/generated.go", struct{}{}); err == nil {
		t.Fatal("dry-run must report invalid templates")
	}
	if err := os.WriteFile(filepath.Join(templateDir, "api.ts.tmpl"), []byte("{{"), 0o644); err != nil {
		t.Fatalf("write frontend template: %v", err)
	}
	frontendGenerator := NewGenerator(&TableSchema{}, "example.com/project", root, true, true, filepath.Join(root, "frontend"))
	if err := frontendGenerator.generateFrontend(&TemplateData{ModuleSnake: "generated"}); err == nil {
		t.Fatal("frontend dry-run must report invalid templates")
	}
}

func renderGeneratorTemplate(t *testing.T, name string, data any) string {
	t.Helper()
	content, err := os.ReadFile("templates/" + name)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	tmpl, err := template.New(name).Funcs(template.FuncMap{"tsType": mapGoToTsType}).Parse(string(content))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("render template: %v", err)
	}
	return out.String()
}

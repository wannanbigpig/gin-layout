package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const baselineDocument = "openapi: 3.0.0\ninfo:\n  version: '0.10.0'\npaths:\n  /example:\n    get:\n      x-security-policy: access_token\n      responses:\n        '200': {description: OK}\n"

func write(t *testing.T, root, name, value string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeRelease(t *testing.T, root, version string, policy Policy) {
	t.Helper()
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "docs/release.json", string(data))
	write(t, root, "CHANGELOG.md", "## ["+version+"]\n\nRelease notes\n")
	var guide strings.Builder
	for _, heading := range []string{"API 变更", "数据库迁移", "配置与队列", "升级顺序", "回滚限制", "验证"} {
		guide.WriteString("## " + heading + "\n\nReviewed release instructions.\n\n")
	}
	write(t, root, "docs/upgrades/"+version+".md", guide.String())
}

func repository(t *testing.T) (string, Policy) {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "release-test@example.invalid"}, {"config", "user.name", "Release Test"}} {
		if _, err := git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	policy := Policy{ChangeType: "initial", CompatibleFrontend: Range{Min: "0.10.0", MaxExclusive: "0.11.0"}, UpgradeGuide: "docs/upgrades/0.10.0.md", APIChanges: []APIChange{}}
	writeRelease(t, root, "0.10.0", policy)
	write(t, root, "docs/openapi.yaml", baselineDocument)
	write(t, root, "docs/error_codes.json", `{"groups":[{"codes":[{"name":"ExampleError","code":20001}]}]}`)
	write(t, root, "data/migrations/20261010000001_example.up.sql", "CREATE TABLE example (id int);\n")
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "initial"}, {"tag", "v0.10.0"}} {
		if _, err := git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	return root, policy
}

func TestInitialReleaseChecksTagIdentityAndCleanTree(t *testing.T) {
	root, _ := repository(t)
	if err := Check(root, "0.10.0", "v0.10.0"); err != nil {
		t.Fatal(err)
	}
	write(t, root, "untracked.txt", "unreleased\n")
	if err := Check(root, "0.10.0", "v0.10.0"); err == nil || !strings.Contains(err.Error(), "clean") {
		t.Fatalf("expected clean checkout error: %v", err)
	}
	if err := Check(root, "0.10.0", "v0.10.1"); err == nil {
		t.Fatal("mismatching tag accepted")
	}
}

func TestPatchDoesNotAllowAPIRemovalOrEditingPublishedMigration(t *testing.T) {
	root, policy := repository(t)
	previous := "v0.10.0"
	policy.PreviousTag, policy.ChangeType, policy.UpgradeGuide = &previous, "patch", "docs/upgrades/0.10.1.md"
	writeRelease(t, root, "0.10.1", policy)
	doc := strings.ReplaceAll(baselineDocument, "0.10.0", "0.10.1")
	write(t, root, "docs/openapi.yaml", doc)
	if err := Check(root, "0.10.1", ""); err != nil {
		t.Fatal(err)
	}
	write(t, root, "data/migrations/20261010000001_example.up.sql", "DROP TABLE example;\n")
	if err := Check(root, "0.10.1", ""); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable migration error: %v", err)
	}
	write(t, root, "data/migrations/20261010000001_example.up.sql", "CREATE TABLE example (id int);\n")
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(doc, "/example", "/renamed"))
	if err := Check(root, "0.10.1", ""); err == nil || !strings.Contains(err.Error(), "requires review") {
		t.Fatalf("expected API diff review error: %v", err)
	}
	policy.APIChanges = []APIChange{{Operation: "GET /example", Kind: "compatible", Reason: "declared"}, {Operation: "GET /renamed", Kind: "compatible", Reason: "declared"}}
	writeRelease(t, root, "0.10.1", policy)
	if err := Check(root, "0.10.1", ""); err == nil {
		t.Fatal("misclassified removal accepted")
	}
	policy.ChangeType = "breaking"
	writeRelease(t, root, "0.10.1", policy)
	if err := Check(root, "0.10.1", ""); err == nil || !strings.Contains(err.Error(), "advance") {
		t.Fatalf("breaking patch accepted: %v", err)
	}
}

func TestBreakingMinorRequiresMatchingDiffAndAllowsAdditiveMigration(t *testing.T) {
	root, policy := repository(t)
	previous := "v0.10.0"
	policy.PreviousTag, policy.ChangeType, policy.UpgradeGuide = &previous, "breaking", "docs/upgrades/0.11.0.md"
	policy.APIChanges = []APIChange{{Operation: "GET /example", Kind: "breaking", Reason: "Replace access token policy; deploy matching client"}}
	writeRelease(t, root, "0.11.0", policy)
	doc := strings.ReplaceAll(strings.ReplaceAll(baselineDocument, "0.10.0", "0.11.0"), "access_token", "access_token_permission")
	write(t, root, "docs/openapi.yaml", doc)
	write(t, root, "data/migrations/20261010000002_add_column.up.sql", "ALTER TABLE example ADD name varchar(50);\n")
	if err := Check(root, "0.11.0", ""); err != nil {
		t.Fatal(err)
	}
	policy.APIChanges = append(policy.APIChanges, APIChange{Operation: "GET /absent", Kind: "compatible", Reason: "not an actual diff"})
	writeRelease(t, root, "0.11.0", policy)
	if err := Check(root, "0.11.0", ""); err == nil {
		t.Fatal("stale review accepted")
	}
}

func TestCannotForgetBaselineOrUseBreakingPatchAfterOne(t *testing.T) {
	root, policy := repository(t)
	policy.UpgradeGuide = "docs/upgrades/0.10.1.md"
	writeRelease(t, root, "0.10.1", policy)
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(baselineDocument, "0.10.0", "0.10.1"))
	if err := Check(root, "0.10.1", ""); err == nil || !strings.Contains(err.Error(), "previousTag") {
		t.Fatalf("missing baseline accepted: %v", err)
	}
	previous := "v0.10.0"
	policy.PreviousTag, policy.ChangeType, policy.UpgradeGuide = &previous, "breaking", "docs/upgrades/1.0.0.md"
	writeRelease(t, root, "1.0.0", policy)
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(baselineDocument, "0.10.0", "1.0.0"))
	if err := Check(root, "1.0.0", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := git(root, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := git(root, "commit", "-qm", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := git(root, "tag", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
	previous = "v1.0.0"
	policy.UpgradeGuide = "docs/upgrades/1.1.0.md"
	writeRelease(t, root, "1.1.0", policy)
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(baselineDocument, "0.10.0", "1.1.0"))
	if err := Check(root, "1.1.0", ""); err == nil {
		t.Fatal("breaking minor after 1.0 accepted")
	}
}

func TestContractChangesIncludesSchemasSecurityAndDeprecation(t *testing.T) {
	for _, scenario := range []struct{ old, new, operation, kind string }{
		{baselineDocument, strings.ReplaceAll(baselineDocument, "access_token", "public"), "GET /example", "breaking"},
		{baselineDocument, strings.ReplaceAll(baselineDocument, "responses:", "deprecated: true\n      responses:"), "GET /example", "review"},
		{baselineDocument, baselineDocument + "components:\n  schemas: {Item: {type: object}}\n", "components", "review"},
		{baselineDocument, baselineDocument + "  /new:\n    post:\n      responses: {'200': {description: OK}}\n", "POST /new", "added"},
	} {
		changes, err := ContractChanges([]byte(scenario.old), []byte(scenario.new))
		if err != nil || changes[scenario.operation] != scenario.kind {
			t.Fatalf("unexpected diff %v, error %v", changes, err)
		}
	}
}

func TestSemanticVersionPrereleaseOrder(t *testing.T) {
	values := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0"}
	for i := 1; i < len(values); i++ {
		a, _ := parseVersion(values[i-1])
		b, _ := parseVersion(values[i])
		if compareVersions(a, b) >= 0 {
			t.Fatalf("incorrect order %s / %s", values[i-1], values[i])
		}
	}
	for _, invalid := range []string{"01.0.0", "1.0.0-01", "1.0.0-rc..1", "v1.0.0", "1.0.0+build"} {
		if _, err := parseVersion(invalid); err == nil {
			t.Fatalf("invalid version accepted: %s", invalid)
		}
	}
}

func TestErrorCodeAndBaselineVersionChecks(t *testing.T) {
	root, policy := repository(t)
	previous := "v0.10.0"
	policy.PreviousTag, policy.ChangeType, policy.UpgradeGuide = &previous, "patch", "docs/upgrades/0.10.1.md"
	writeRelease(t, root, "0.10.1", policy)
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(baselineDocument, "0.10.0", "0.10.1"))
	write(t, root, "docs/error_codes.json", `{"groups":[]}`)
	if err := Check(root, "0.10.1", ""); err == nil || !strings.Contains(err.Error(), "error-code ExampleError") {
		t.Fatalf("removed error code accepted: %v", err)
	}
	policy.APIChanges = []APIChange{{Operation: "error-code ExampleError", Kind: "compatible", Reason: "removed"}}
	writeRelease(t, root, "0.10.1", policy)
	if err := Check(root, "0.10.1", ""); err == nil {
		t.Fatal("compatible error removal accepted")
	}
	if _, err := git(root, "tag", "v0.10.9"); err != nil {
		t.Fatal(err)
	}
	previous = "v0.10.9"
	policy.ChangeType, policy.UpgradeGuide = "breaking", "docs/upgrades/0.11.0.md"
	writeRelease(t, root, "0.11.0", policy)
	write(t, root, "docs/openapi.yaml", strings.ReplaceAll(baselineDocument, "0.10.0", "0.11.0"))
	if err := Check(root, "0.11.0", ""); err == nil || !strings.Contains(err.Error(), "previous tag") {
		t.Fatalf("mislabeled baseline accepted: %v", err)
	}
}

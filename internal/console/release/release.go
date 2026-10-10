package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

type Range struct {
	Min          string `json:"min"`
	MaxExclusive string `json:"maxExclusive"`
}

type APIChange struct {
	Operation string `json:"operation"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
}

type Policy struct {
	PreviousTag        *string     `json:"previousTag"`
	ChangeType         string      `json:"changeType"`
	CompatibleFrontend Range       `json:"compatibleFrontend"`
	UpgradeGuide       string      `json:"upgradeGuide"`
	APIChanges         []APIChange `json:"apiChanges"`
}

var Cmd = &cobra.Command{
	Use:   "release-check",
	Short: "Check release version, API changes and immutable migrations without database access",
	RunE: func(cmd *cobra.Command, _ []string) error {
		root, _ := cmd.Flags().GetString("root")
		tag, _ := cmd.Flags().GetString("tag")
		if err := Check(root, global.Version, tag); err != nil {
			return err
		}
		cmd.Println("Release metadata and available API contract checks passed; business/schema review is required in the upgrade guide.")
		return nil
	},
}

func init() {
	Cmd.Flags().String("root", ".", "Repository root")
	Cmd.Flags().String("tag", "", "Release tag; require a clean checkout at this tag with full tag history")
}

func git(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, output)
	}
	return output, nil
}

func Check(root, version, tag string) error {
	current, err := parseVersion(version)
	if err != nil {
		return err
	}
	shallow, err := git(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(shallow)) == "true" {
		return fmt.Errorf("release checks require full Git history and tags")
	}
	data, err := os.ReadFile(filepath.Join(root, "docs/release.json"))
	if err != nil {
		return err
	}
	var policy Policy
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if _, exists := fields["previousTag"]; !exists {
		return fmt.Errorf("previousTag is required; null only for first release")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("release.json must contain exactly one JSON object")
	}
	minimum, err := parseVersion(policy.CompatibleFrontend.Min)
	if err != nil {
		return err
	}
	maximum, err := parseVersion(policy.CompatibleFrontend.MaxExclusive)
	if err != nil || compareVersions(minimum, maximum) >= 0 {
		return fmt.Errorf("compatibleFrontend must be a nonempty min/maxExclusive range")
	}
	if policy.ChangeType != "initial" && policy.ChangeType != "patch" && policy.ChangeType != "feature" && policy.ChangeType != "breaking" {
		return fmt.Errorf("invalid changeType")
	}
	guidePath := filepath.Clean(policy.UpgradeGuide)
	if guidePath != filepath.Join("docs", "upgrades", version+".md") {
		return fmt.Errorf("upgradeGuide must be docs/upgrades/%s.md", version)
	}
	guide, err := os.ReadFile(filepath.Join(root, guidePath))
	if err != nil {
		return err
	}
	for _, heading := range []string{"API 变更", "数据库迁移", "配置与队列", "升级顺序", "回滚限制", "验证"} {
		_, body, found := strings.Cut(string(guide), "## "+heading+"\n")
		body, _, _ = strings.Cut(body, "\n## ")
		if !found || strings.TrimSpace(body) == "" {
			return fmt.Errorf("upgrade guide requires nonempty section %s", heading)
		}
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(changelog), "## ["+version+"]") {
		return fmt.Errorf("CHANGELOG is missing version %s", version)
	}
	currentDoc, err := os.ReadFile(filepath.Join(root, "docs/openapi.yaml"))
	if err != nil {
		return err
	}
	doc, err := readDocument(currentDoc)
	if err != nil {
		return err
	}
	info, _ := doc["info"].(map[string]any)
	if info["version"] != version {
		return fmt.Errorf("OpenAPI info.version differs from source version")
	}
	tags, err := git(root, "tag", "--list")
	if err != nil {
		return err
	}
	if policy.PreviousTag == nil {
		for _, existing := range strings.Fields(string(tags)) {
			if existing == "v"+version {
				continue
			}
			if strings.HasPrefix(existing, "v") {
				if _, err := parseVersion(existing[1:]); err == nil {
					return fmt.Errorf("previousTag cannot be null after release tags exist")
				}
			}
		}
	}
	if tag != "" {
		if tag != "v"+version {
			return fmt.Errorf("tag differs from source version")
		}
		status, err := git(root, "status", "--porcelain")
		if err != nil {
			return err
		}
		if len(status) != 0 {
			return fmt.Errorf("release requires a clean checkout")
		}
		pointsAt, err := git(root, "tag", "--points-at", "HEAD")
		if err != nil {
			return err
		}
		if !containsLine(string(pointsAt), tag) {
			return fmt.Errorf("checkout must be at release tag %s", tag)
		}
	}
	if policy.PreviousTag == nil {
		if policy.ChangeType != "initial" || len(policy.APIChanges) != 0 {
			return fmt.Errorf("first release must use initial and no previous API changes")
		}
		return nil
	}
	if policy.ChangeType == "initial" {
		return fmt.Errorf("initial cannot have previousTag")
	}
	if !strings.HasPrefix(*policy.PreviousTag, "v") {
		return fmt.Errorf("previousTag must start with v")
	}
	before, err := parseVersion((*policy.PreviousTag)[1:])
	if err != nil {
		return err
	}
	if compareVersions(current, before) <= 0 {
		return fmt.Errorf("version must advance beyond previousTag")
	}
	if _, err := git(root, "merge-base", "--is-ancestor", *policy.PreviousTag, "HEAD"); err != nil {
		return fmt.Errorf("previousTag must be an ancestor: %w", err)
	}
	for _, existing := range strings.Fields(string(tags)) {
		if existing == "v"+version || !strings.HasPrefix(existing, "v") {
			continue
		}
		candidate, parseErr := parseVersion(existing[1:])
		if parseErr == nil && compareVersions(candidate, before) > 0 {
			if _, ancestorErr := git(root, "merge-base", "--is-ancestor", existing, "HEAD"); ancestorErr == nil {
				return fmt.Errorf("previousTag must be the latest version tag on this history: %s", existing)
			}
		}
	}
	if policy.ChangeType == "breaking" && current.core[0] == before.core[0] && (before.core[0] != 0 || current.core[1] == before.core[1]) {
		return fmt.Errorf("breaking change requires major advance, or minor advance while 0.x")
	}
	if policy.ChangeType == "feature" && current.core[0] == before.core[0] && current.core[1] == before.core[1] {
		return fmt.Errorf("feature requires at least minor advance")
	}
	if policy.ChangeType == "patch" && (current.core[0] != before.core[0] || current.core[1] != before.core[1]) {
		return fmt.Errorf("patch must stay in the same major/minor")
	}
	baseline, err := git(root, "show", *policy.PreviousTag+":docs/openapi.yaml")
	if err != nil {
		return err
	}
	changes, err := ContractChanges(baseline, currentDoc)
	if err != nil {
		return err
	}
	baselineDoc, _ := readDocument(baseline)
	baselineInfo, _ := baselineDoc["info"].(map[string]any)
	if baselineInfo["version"] != (*policy.PreviousTag)[1:] {
		return fmt.Errorf("previous tag does not match its OpenAPI version")
	}
	oldCodes, err := git(root, "show", *policy.PreviousTag+":docs/error_codes.json")
	if err != nil {
		return err
	}
	nowCodes, err := os.ReadFile(filepath.Join(root, "docs/error_codes.json"))
	if err != nil {
		return err
	}
	codeChanges, err := errorCodeChanges(oldCodes, nowCodes)
	if err != nil {
		return err
	}
	for key, kind := range codeChanges {
		changes[key] = kind
	}
	reviews := make(map[string]APIChange)
	for _, review := range policy.APIChanges {
		if (review.Kind != "compatible" && review.Kind != "breaking") || strings.TrimSpace(review.Reason) == "" {
			return fmt.Errorf("apiChanges needs compatible/breaking and a reason")
		}
		if _, exists := reviews[review.Operation]; exists {
			return fmt.Errorf("duplicate API review %s", review.Operation)
		}
		reviews[review.Operation] = review
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, operation := range keys {
		kind := changes[operation]
		review, ok := reviews[operation]
		if !ok {
			return fmt.Errorf("API change requires review: %s (%s)", operation, kind)
		}
		if kind == "added" && policy.ChangeType == "patch" {
			return fmt.Errorf("new operation requires feature or breaking release")
		}
		if kind == "breaking" && review.Kind != "breaking" {
			return fmt.Errorf("removed/authentication-changed operation cannot be compatible: %s", operation)
		}
		if review.Kind == "breaking" && policy.ChangeType != "breaking" {
			return fmt.Errorf("breaking API change requires changeType breaking")
		}
		delete(reviews, operation)
	}
	if len(reviews) != 0 {
		return fmt.Errorf("apiChanges contains entries absent from the actual diff")
	}
	files, err := git(root, "ls-tree", "-r", "--name-only", *policy.PreviousTag, "--", "data/migrations")
	if err != nil {
		return err
	}
	for _, name := range strings.Split(strings.TrimSpace(string(files)), "\n") {
		if name == "" {
			continue
		}
		old, err := git(root, "show", *policy.PreviousTag+":"+name)
		if err != nil {
			return err
		}
		now, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(old, now) {
			return fmt.Errorf("published migration is immutable: %s", name)
		}
	}
	return nil
}

func containsLine(value, expected string) bool {
	for _, line := range strings.Split(value, "\n") {
		if line == expected {
			return true
		}
	}
	return false
}

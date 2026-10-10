package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?$`)

type semVersion struct {
	core [3]int
	pre  string
}

func parseVersion(value string) (semVersion, error) {
	var result semVersion
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return result, fmt.Errorf("invalid release version %q", value)
	}
	for i := range result.core {
		n, err := strconv.Atoi(match[i+1])
		if err != nil {
			return result, err
		}
		result.core[i] = n
	}
	result.pre = match[4]
	if result.pre != "" {
		for _, part := range strings.Split(result.pre, ".") {
			if part == "" || (isNumeric(part) && len(part) > 1 && part[0] == '0') {
				return result, fmt.Errorf("invalid prerelease identifier %q", part)
			}
		}
	}
	return result, nil
}

func isNumeric(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func compareVersions(a, b semVersion) int {
	for i := range a.core {
		if a.core[i] < b.core[i] {
			return -1
		}
		if a.core[i] > b.core[i] {
			return 1
		}
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	aa, bb := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] == bb[i] {
			continue
		}
		an, bn := isNumeric(aa[i]), isNumeric(bb[i])
		if an && bn && len(aa[i]) != len(bb[i]) {
			if len(aa[i]) < len(bb[i]) {
				return -1
			}
			return 1
		}
		if an != bn {
			if an {
				return -1
			}
			return 1
		}
		if aa[i] < bb[i] {
			return -1
		}
		return 1
	}
	if len(aa) < len(bb) {
		return -1
	}
	return 1
}

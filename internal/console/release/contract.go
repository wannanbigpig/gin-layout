package release

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func errorCodeChanges(before, after []byte) (map[string]string, error) {
	read := func(data []byte) (map[string]int, error) {
		var document struct {
			Groups []struct {
				Codes []struct {
					Name string
					Code int
				}
			}
		}
		if err := json.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		result := make(map[string]int)
		for _, group := range document.Groups {
			for _, code := range group.Codes {
				if code.Name == "" {
					return nil, fmt.Errorf("empty error code name")
				}
				if _, exists := result[code.Name]; exists {
					return nil, fmt.Errorf("duplicate error code name")
				}
				result[code.Name] = code.Code
			}
		}
		return result, nil
	}
	old, err := read(before)
	if err != nil {
		return nil, err
	}
	now, err := read(after)
	if err != nil {
		return nil, err
	}
	changes := make(map[string]string)
	for name, code := range old {
		if value, exists := now[name]; !exists || code != value {
			changes["error-code "+name] = "breaking"
		}
	}
	for name := range now {
		if _, exists := old[name]; !exists {
			changes["error-code "+name] = "review"
		}
	}
	return changes, nil
}

func readDocument(data []byte) (map[string]any, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok || len(paths) == 0 {
		return nil, fmt.Errorf("OpenAPI paths must not be empty")
	}
	return doc, nil
}

// Operation/schema differences require a human classification. The current generator
// describes routes/security; business fields and error semantics also need manual review.
func ContractChanges(before, after []byte) (map[string]string, error) {
	old, err := readDocument(before)
	if err != nil {
		return nil, err
	}
	now, err := readDocument(after)
	if err != nil {
		return nil, err
	}
	changes := make(map[string]string)
	oldPaths := old["paths"].(map[string]any)
	newPaths := now["paths"].(map[string]any)
	methods := []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"}
	for path, raw := range oldPaths {
		oldPath, _ := raw.(map[string]any)
		newPath, _ := newPaths[path].(map[string]any)
		for _, method := range methods {
			oldOp, exists := oldPath[method]
			if !exists {
				continue
			}
			key := strings.ToUpper(method) + " " + path
			newOp, exists := newPath[method]
			if !exists {
				changes[key] = "breaking"
				continue
			}
			a, _ := oldOp.(map[string]any)
			b, _ := newOp.(map[string]any)
			oldSecurity, newSecurity := a["security"], b["security"]
			if oldSecurity == nil {
				oldSecurity = old["security"]
			}
			if newSecurity == nil {
				newSecurity = now["security"]
			}
			if !reflect.DeepEqual(oldSecurity, newSecurity) || !reflect.DeepEqual(a["x-security-policy"], b["x-security-policy"]) {
				changes[key] = "breaking"
				continue
			}
			for _, field := range []string{"parameters", "requestBody", "responses", "deprecated", "servers"} {
				if !reflect.DeepEqual(a[field], b[field]) {
					changes[key] = "review"
				}
			}
			if !reflect.DeepEqual(oldPath["parameters"], newPath["parameters"]) || !reflect.DeepEqual(oldPath["servers"], newPath["servers"]) {
				changes[key] = "review"
			}
		}
	}
	for path, raw := range newPaths {
		newPath, _ := raw.(map[string]any)
		oldPath, _ := oldPaths[path].(map[string]any)
		for _, method := range methods {
			if _, exists := newPath[method]; exists {
				if _, existed := oldPath[method]; !existed {
					changes[strings.ToUpper(method)+" "+path] = "added"
				}
			}
		}
	}
	if !reflect.DeepEqual(old["components"], now["components"]) {
		changes["components"] = "review"
	}
	if !reflect.DeepEqual(old["servers"], now["servers"]) {
		changes["servers"] = "review"
	}
	return changes, nil
}

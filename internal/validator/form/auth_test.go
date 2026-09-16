package form

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoginCredentialLengthMatchesCreation(t *testing.T) {
	for _, tc := range []struct {
		username, password int
		valid              bool
	}{{20, 32, true}, {17, 19, true}, {21, 32, false}, {20, 33, false}} {
		body, _ := json.Marshal(map[string]string{"username": strings.Repeat("u", tc.username), "password": strings.Repeat("p", tc.password), "captcha": "1234", "captcha_id": "test"})
		err := bindJSONBody(t, string(body), NewLoginForm())
		if (err == nil) != tc.valid {
			t.Fatalf("username %d password %d: %v", tc.username, tc.password, err)
		}
	}
}

package sensitive_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

func TestMixedCredentialValuesCannotBypassMasking(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	sensitive.LoadSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{})
	const credential = "CredentialFixtureOnly"
	for name, suffix := range map[string]string{"phone": "; mobile=13800138000", "email": "; email=test@example.com"} {
		t.Run(name, func(t *testing.T) {
			value := credential + suffix
			body, _ := json.Marshal(map[string]string{"password": value, "api_key": value})
			for scope, output := range map[string]string{
				"request_header":  sensitive.GetMaskedRequestHeaders(http.Header{"Cookie": {"refresh_token=" + value}}),
				"response_header": sensitive.GetMaskedResponseHeaders(http.Header{"Set-Cookie": {"refresh_token=" + value}}),
				"request_body":    sensitive.GetMaskedRequestBody(body, "application/json"),
				"response_body":   sensitive.GetMaskedResponseBody(body),
				"query":           sensitive.MaskQueryString(url.Values{"password": {value}}.Encode()),
				"change_diff":     auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{"password": value}, []auditdiff.FieldRule{{Field: "password"}})),
			} {
				if strings.Contains(output, credential) {
					t.Errorf("%s leaked credential when mixed with %s", scope, name)
				}
			}
		})
	}
}

func TestMalformedQueryDoesNotLogCredentials(t *testing.T) {
	output := sensitive.MaskQueryString("password=CredentialFixtureOnly&bad=%zz")
	if strings.Contains(output, "CredentialFixtureOnly") {
		t.Fatal("invalid query exposed credentials")
	}
}

func TestConcurrentCredentialMaskingAndConfigReload(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			sensitive.LoadSensitiveFieldsConfig(sensitive.DefaultSensitiveFieldsConfig())
			sensitive.LoadSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			body := []byte(`{"password":"CredentialFixtureOnly"}`)
			headers := http.Header{"Authorization": {"Bearer CredentialFixtureOnly"}}
			for _, output := range []string{sensitive.GetMaskedRequestHeaders(headers), sensitive.GetMaskedResponseHeaders(headers), sensitive.GetMaskedRequestBody(body, "application/json"), sensitive.GetMaskedResponseBody(body), sensitive.MaskQueryString("password=CredentialFixtureOnly")} {
				if strings.Contains(output, "CredentialFixtureOnly") {
					t.Error("concurrent config reload removed credential baseline")
				}
			}
			_ = sensitive.GetSensitiveFieldsConfig()
		}
	}()
	wg.Wait()
}

func TestCredentialBaselineSurvivesRuntimeConfigReplacement(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	removedSecret := sensitive.DefaultSensitiveFieldsConfig()
	for i, key := range removedSecret.Common {
		if key == "secret" {
			removedSecret.Common = append(removedSecret.Common[:i], removedSecret.Common[i+1:]...)
			break
		}
	}
	removedCookie := sensitive.DefaultSensitiveFieldsConfig()
	for i, key := range removedCookie.RequestHeader {
		if key == "cookie" {
			removedCookie.RequestHeader = append(removedCookie.RequestHeader[:i], removedCookie.RequestHeader[i+1:]...)
			break
		}
	}
	for name, cfg := range map[string]sensitive.SensitiveFieldsConfig{"empty": {}, "only_business": {Common: []string{"private_note"}}, "remove_secret": removedSecret, "remove_cookie": removedCookie} {
		t.Run(name, func(t *testing.T) {
			sensitive.LoadSensitiveFieldsConfig(cfg)
			fixture := "CredentialFixtureOnly"
			body := []byte(`{"password":"` + fixture + `","nested":{"CURRENT_PASSWORD":"` + fixture + `","secret":"` + fixture + `","access_token":"` + fixture + `","refresh_token":"` + fixture + `"},"name":"visible-name"}`)
			headers := http.Header{"Authorization": []string{"Bearer " + fixture}, "Cookie": []string{"refresh_token=" + fixture}, "Set-Cookie": []string{"refresh_token=" + fixture + "; HttpOnly"}, "X-Request-Id": []string{"visible-request"}}
			query := url.Values{"password": {fixture}, "access_token": {fixture}, "refresh_token": {fixture}, "name": {"visible-name"}}.Encode()
			diffs := auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{"password": fixture, "secret": fixture, "access_token": fixture, "name": "visible-name"}, []auditdiff.FieldRule{{Field: "password"}, {Field: "secret"}, {Field: "access_token"}, {Field: "name"}}))
			for scope, result := range map[string]string{
				"request":          sensitive.GetMaskedRequestBody(body, "application/json"),
				"response":         sensitive.GetMaskedResponseBody(body),
				"request_headers":  sensitive.GetMaskedRequestHeaders(headers),
				"response_headers": sensitive.GetMaskedResponseHeaders(headers),
				"query":            sensitive.MaskQueryString(query),
				"form":             sensitive.GetMaskedRequestBody([]byte(query), "application/x-www-form-urlencoded"),
				"change_diff":      diffs,
			} {
				if strings.Contains(result, fixture) {
					t.Errorf("%s still contains an authentication credential", scope)
				}
				if !strings.Contains(result, "visible-name") && !strings.Contains(result, "visible-request") {
					t.Errorf("%s lost ordinary business data: %s", scope, result)
				}
			}
		})
	}
}

func TestCredentialBaselinePreservesBusinessFieldConfiguration(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	body := []byte(`{"password":"CredentialFixtureOnly","business_note":"BusinessFixtureOnly","shipping":"ordinary-shipping","compass":"ordinary-compass","tokenizer":"ordinary-tokenizer"}`)
	for _, maskBusiness := range []bool{true, false} {
		cfg := sensitive.SensitiveFieldsConfig{}
		if maskBusiness {
			cfg.Common = []string{"business_note"}
		}
		sensitive.LoadSensitiveFieldsConfig(cfg)
		output := sensitive.GetMaskedResponseBody(body)
		if strings.Contains(output, "CredentialFixtureOnly") {
			t.Error("credential baseline removed")
		}
		if strings.Contains(output, "BusinessFixtureOnly") == maskBusiness {
			t.Fatalf("business field configuration was ignored: %s", output)
		}
		for _, business := range []string{"ordinary-shipping", "ordinary-compass", "ordinary-tokenizer"} {
			if !strings.Contains(output, business) {
				t.Errorf("immutable baseline masked unrelated business field %s", business)
			}
		}
		actual, _ := json.Marshal(sensitive.GetSensitiveFieldsConfig())
		if strings.Contains(string(actual), "password") {
			t.Fatalf("immutable baseline must not replace editable configuration: %s", actual)
		}
	}
}

func TestRegisteredProviderCredentialSurvivesEmptyConfigAcrossAuditScopes(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	sensitive.RegisterImmutableRequestBodyFields("provider_fixture_private_material")
	sensitive.LoadSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{})
	fixture := "ProviderFixtureOnly"
	payload := []byte(`{"providers":[{"provider_fixture_private_material":"` + fixture + `"}]}`)
	for scope, result := range map[string]string{
		"request":     sensitive.GetMaskedRequestBody(payload, "application/json"),
		"response":    sensitive.GetMaskedResponseBody(payload),
		"query":       sensitive.MaskQueryString("provider_fixture_private_material=" + fixture),
		"change_diff": auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{"provider_fixture_private_material": fixture}, []auditdiff.FieldRule{{Field: "provider_fixture_private_material"}})),
	} {
		if strings.Contains(result, fixture) {
			t.Errorf("%s leaked registered provider credential", scope)
		}
	}
}

package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestAuditSnapshotOmitsUnstructuredCredentialBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const credential = "CredentialFixtureOnly"
	for _, test := range []struct {
		contentType string
		body        string
		binds       bool
	}{
		{"application/xml", `<LoginAuth><UserName>admin</UserName><PassWord>` + credential + `</PassWord><Captcha>fake</Captcha><CaptchaID>fake</CaptchaID></LoginAuth>`, true},
		{"application/x-yaml", "username: admin\npassword: " + credential + "\ncaptcha: fake\ncaptcha_id: fake\n", true},
		{"application/toml", "UserName = \"admin\"\nPassWord = \"" + credential + "\"\nCaptcha = \"fake\"\nCaptchaID = \"fake\"\n", true},
		{"text/plain", "password=" + credential, false},
	} {
		t.Run(test.contentType, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/login", bytes.NewBufferString(test.body))
			ctx.Request.Header.Set("Content-Type", test.contentType)
			ctx.Set(global.ContextKeyRequestID, "unstructured-credential-test")
			ctx.Set(global.ContextKeyOperationName, "login fixture")
			cacheRequestBody(ctx)
			if test.binds {
				var params form.LoginAuth
				if err := ctx.ShouldBind(&params); err != nil || params.PassWord != credential {
					t.Fatalf("credential fixture did not bind: %v", err)
				}
			}
			recorder := createResponseRecorder(ctx)
			snapshot := buildRequestAuditLogSnapshot(ctx, recorder, &response.Result{Code: 400})
			if snapshot == nil || strings.Contains(snapshot.RequestBody, credential) || !strings.Contains(snapshot.RequestBody, "body not logged") {
				t.Fatalf("unstructured request body was not safely omitted: %#v", snapshot)
			}
		})
	}
}

func TestAuditSnapshotKeepsCredentialBaselineWithEmptyConfig(t *testing.T) {
	previous := sensitive.GetSensitiveFieldsConfig()
	t.Cleanup(func() { sensitive.LoadSensitiveFieldsConfig(previous) })
	sensitive.LoadSensitiveFieldsConfig(sensitive.SensitiveFieldsConfig{})
	gin.SetMode(gin.TestMode)

	const credential = "CredentialFixtureOnly"
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/login?access_token="+credential+"&name=visible-name", bytes.NewBufferString(`{"password":"`+credential+`","name":"visible-name"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Authorization", "Bearer "+credential)
	ctx.Request.Header.Set("Cookie", "refresh_token="+credential)
	ctx.Request.Header.Set("X-Request-ID", "visible-request")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "credential-baseline-test")
	ctx.Set(global.ContextKeyOperationName, "login fixture")
	cacheRequestBody(ctx)
	SetAuditChangeDiffRaw(ctx, auditdiff.Marshal(auditdiff.BuildFieldDiff(nil, map[string]any{"password": credential, "name": "visible-name"}, []auditdiff.FieldRule{{Field: "password"}, {Field: "name"}})))

	recorder := createResponseRecorder(ctx)
	recorder.Header().Set("Set-Cookie", "refresh_token="+credential+"; HttpOnly")
	recorder.Header().Set("X-Request-ID", "visible-request")
	recorder.body.WriteString(`{"code":0,"data":{"access_token":"` + credential + `","name":"visible-name"}}`)
	snapshot := buildRequestAuditLogSnapshot(ctx, recorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	for scope, output := range map[string]string{
		"request_body": snapshot.RequestBody, "request_headers": snapshot.RequestHeaders,
		"response_body": snapshot.ResponseBody, "response_headers": snapshot.ResponseHeader,
		"query": snapshot.RequestQuery, "change_diff": snapshot.ChangeDiff,
	} {
		if strings.Contains(output, credential) {
			t.Errorf("%s snapshot contains an authentication credential", scope)
		}
		if !strings.Contains(output, "visible-name") && !strings.Contains(output, "visible-request") {
			t.Errorf("%s snapshot lost ordinary data: %s", scope, output)
		}
	}
}

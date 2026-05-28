package errors

import (
	"net/http"
	"testing"
)

// TestInferHTTPStatus 覆盖契约：
//   - SUCCESS=200
//   - 协议级显式映射稳定
//   - 服务端故障类 code 显式 500（不被 fallback 误归 4xx）
//   - 未登记业务码默认 400（fallback 翻转后的新契约）
func TestInferHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		// ---- 成功 ----
		{"SUCCESS -> 200", SUCCESS, http.StatusOK},

		// ---- 协议级映射 ----
		{"NotLogin -> 401", NotLogin, http.StatusUnauthorized},
		{"AuthorizationErr -> 403", AuthorizationErr, http.StatusForbidden},
		{"NotFound -> 404", NotFound, http.StatusNotFound},
		{"ServerErr -> 500", ServerErr, http.StatusInternalServerError},
		{"ServiceDependencyNotReady -> 503", ServiceDependencyNotReady, http.StatusServiceUnavailable},
		{"InvalidParameter -> 400", InvalidParameter, http.StatusBadRequest},
		{"CaptchaErr -> 400", CaptchaErr, http.StatusBadRequest},

		// ---- 登录凭证类（前端 credential 分流契约依赖） ----
		{"UserPasswordWrong -> 401", UserPasswordWrong, http.StatusUnauthorized},
		{"UserDoesNotExist -> 401", UserDoesNotExist, http.StatusUnauthorized},
		{"LoginAccountLocked -> 401", LoginAccountLocked, http.StatusUnauthorized},

		// ---- 服务端技术性故障显式 500 ----
		{"CasbinInitFailed -> 500", CasbinInitFailed, http.StatusInternalServerError},
		{"TokenGenerateFailed -> 500", TokenGenerateFailed, http.StatusInternalServerError},
		{"PasswordProcessFailed -> 500", PasswordProcessFailed, http.StatusInternalServerError},
		{"LoginFailed -> 500", LoginFailed, http.StatusInternalServerError},
		{"CreateUserFailed -> 500", CreateUserFailed, http.StatusInternalServerError},
		{"UpdateUserFailed -> 500", UpdateUserFailed, http.StatusInternalServerError},
		{"DeleteUserFailed -> 500", DeleteUserFailed, http.StatusInternalServerError},
		{"QueryUserDeptFailed -> 500", QueryUserDeptFailed, http.StatusInternalServerError},

		// ---- fallback：未登记业务码默认 400 ----
		{"unknown code -> 400", 99999, http.StatusBadRequest},
		{"FAILURE -> 400", FAILURE, http.StatusBadRequest},
		{"TooManyRequests -> 400", TooManyRequests, http.StatusBadRequest},
		{"UserDisable -> 400", UserDisable, http.StatusBadRequest},
		{"negative code -> 400", -1, http.StatusBadRequest},

		// ---- 客户端类业务码走 fallback（覆盖典型业务域） ----
		{"FileReferenced -> 400", FileReferenced, http.StatusBadRequest},
		{"FileTooLarge -> 400", FileTooLarge, http.StatusBadRequest},
		{"FileMimeTypeNotAllowed -> 400", FileMimeTypeNotAllowed, http.StatusBadRequest},
		{"RoleExists -> 400", RoleExists, http.StatusBadRequest},
		{"RoleCannotDelete -> 400", RoleCannotDelete, http.StatusBadRequest},
		{"MenuNotFound -> 400", MenuNotFound, http.StatusBadRequest},
		{"DepartmentHasChildren -> 400", DepartmentHasChildren, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferHTTPStatus(tt.code)
			if got != tt.want {
				t.Errorf("InferHTTPStatus(%d) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

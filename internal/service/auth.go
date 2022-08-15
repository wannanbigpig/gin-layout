package service

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	err "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

type AuthService struct {
}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (auth *AuthService) Login(username, password string) (*model.AdminUsers, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	user := adminUsersModel.GetUserInfo(username)

	if user == nil {
		berr := err.NewBusinessError(err.UserDoesNotExist)
		return nil, berr
	}

	// 校验密码
	if !adminUsersModel.ComparePasswords(password) {
		return nil, err.NewBusinessError(err.FAILURE, "用户密码错误")
	}

	/* TODO 生成 token 等业务逻辑，此处不再演示，直接返回用户信息 */
	// ...

	return user, nil
}

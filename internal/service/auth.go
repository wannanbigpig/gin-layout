package service

import (
	"errors"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func Login(username, password string) (*model.AdminUsers, error) {
	// 查询用户是否存在
	adminUsersModel := model.NewAdminUsers()
	user := adminUsersModel.GetUserInfo(username)

	if user == nil {
		return nil, errors.New("用户不存在")
	}

	// 校验密码
	if !adminUsersModel.ComparePasswords(password) {
		return nil, errors.New("用户密码错误")
	}

	/* TODO 生成 token 等业务逻辑，此处不再演示，直接返回用户信息 */
	// ...

	return user, nil
}

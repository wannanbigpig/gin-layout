package service

import (
	"errors"
	"l-admin.com/internal/model"
)

func Login(username, password string) (*model.AdminUsers, error) {
	user := model.AdminUsersModel().GetUserInfo(username)
	if user == nil {
		return nil, errors.New("用户不存在")
	}
	return user, nil
}

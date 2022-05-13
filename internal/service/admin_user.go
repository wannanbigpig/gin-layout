package service

import "github.com/wannanbigpig/gin-layout/internal/model"

func GetUserInfo(id uint) *model.AdminUsers {
	return model.AdminUsersModel().GetUserById(id)
}

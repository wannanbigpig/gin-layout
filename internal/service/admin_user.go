package service

import "l-admin.com/internal/model"

func GetUserInfo(id uint) *model.AdminUsers {
	return model.AdminUsersModel().GetUserById(id)
}

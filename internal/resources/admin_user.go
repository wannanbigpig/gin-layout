package resources

import "github.com/wannanbigpig/gin-layout/internal/model"

type AdminUserResources struct {
	ID       uint   `json:"id"`
	Nickname string `json:"nickname"`
	Username string `json:"username"`
	IsAdmin  int8   `json:"is_admin"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

func NewAdminUserResources(user *model.AdminUsers) *AdminUserResources {
	return &AdminUserResources{
		ID:       user.ID,
		Nickname: user.Nickname,
		Username: user.Username,
		Mobile:   user.Mobile,
		IsAdmin:  user.IsAdmin,
		Email:    user.Email,
		Avatar:   user.Avatar,
	}
}

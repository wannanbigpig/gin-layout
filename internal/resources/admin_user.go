package resources

import (
	"github.com/jinzhu/copier"
)

type AdminUserResources struct {
	ID       uint   `json:"id"`
	Nickname string `json:"nickname"`
	Username string `json:"username"`
	IsAdmin  int8   `json:"is_admin"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
}

func NewAdminUserResources(data any) *AdminUserResources {
	var adminUser AdminUserResources
	err := copier.Copy(&adminUser, data)
	if err != nil {
		return nil
	}
	return &adminUser
}

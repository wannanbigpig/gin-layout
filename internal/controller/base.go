package controller

import (
	response2 "l-admin.com/internal/pkg/response"
)

func resp() *response2.Response {
	// 初始化response
	return response2.NewResponse()
}

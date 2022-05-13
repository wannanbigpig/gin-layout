package controller

import (
	response2 "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

func resp() *response2.Response {
	// 初始化response
	return response2.NewResponse()
}

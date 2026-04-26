package api_permission

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var apiPermissionDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "接口ID"},
	{Field: "name", Label: "接口名称"},
	{Field: "description", Label: "描述"},
	{
		Field: "is_auth",
		Label: "鉴权模式",
		ValueLabels: map[string]string{
			"0": "无需登录",
			"1": "需要登录",
			"2": "需要登录且鉴权",
		},
	},
	{Field: "sort", Label: "排序"},
}

// UpdateWithAuditDiff 更新 API 权限并返回精确 change_diff。
func (s *ApiService) UpdateWithAuditDiff(params *form.UpdatePermission) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotAPIPermissionByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.Update(params); err != nil {
		return "", err
	}
	after, err := s.snapshotAPIPermissionByID(params.Id)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	items := auditdiff.BuildFieldDiff(before, after, apiPermissionDiffRules)
	return auditdiff.Marshal(items), nil
}

func (s *ApiService) snapshotAPIPermissionByID(id uint) (map[string]any, error) {
	apiModel := model.NewApi()
	if err := apiModel.GetById(id); err != nil || apiModel.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return map[string]any{
		"id":          apiModel.ID,
		"name":        apiModel.Name,
		"description": apiModel.Description,
		"is_auth":     apiModel.IsAuth,
		"sort":        apiModel.Sort,
	}, nil
}

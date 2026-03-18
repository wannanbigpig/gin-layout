package routers

import "github.com/wannanbigpig/gin-layout/pkg/utils"

// CollectAdminRouteMeta 收集当前应用的全部路由元数据。
func CollectAdminRouteMeta() RouteMetaMap {
	return CollectRouteMeta(AppRouteTree())
}

// CollectRouteMeta 根据路由树递归收集路由元数据。
func CollectRouteMeta(root RouteGroupDef) RouteMetaMap {
	metaMap := make(RouteMetaMap)
	collectRouteMeta(metaMap, root, "", "")
	return metaMap
}

func collectRouteMeta(metaMap RouteMetaMap, group RouteGroupDef, basePath, inheritedGroupCode string) {
	fullPrefix := joinFullPath(basePath, group.Prefix)
	groupCode := inheritedGroupCode
	if group.GroupCode != "" {
		groupCode = group.GroupCode
	}

	for _, route := range group.Routes {
		fullPath := joinFullPath(fullPrefix, route.Path)
		meta := &RouteMeta{
			Method:    route.Method,
			Path:      fullPath,
			Title:     route.Title,
			Desc:      route.Desc,
			Auth:      route.Auth,
			GroupCode: groupCode,
		}
		metaMap[utils.MD5(meta.Method+"_"+meta.Path)] = meta
	}

	for _, child := range group.Children {
		collectRouteMeta(metaMap, child, fullPrefix, groupCode)
	}
}

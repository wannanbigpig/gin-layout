package query_builder

import "strings"

// ResolveListOrder 只允许调用方声明的数据库列参与排序，并用 id 保证跨页顺序稳定。
func ResolveListOrder(sortBy, sortOrder, defaultOrder string, allowed ...string) string {
	if sortBy == "" || (sortOrder != "asc" && sortOrder != "desc") {
		return defaultOrder
	}
	for _, field := range allowed {
		if sortBy == field {
			order := field + " " + strings.ToUpper(sortOrder)
			if field != "id" {
				order += ", id DESC"
			}
			return order
		}
	}
	return defaultOrder
}

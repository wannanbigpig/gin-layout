package modelDict

import "github.com/wannanbigpig/gin-layout/internal/global"

type Dict map[int8]string

func (d Dict) Map(k int8) string {
	// 先判断 d 是否为 nil，防止 nil 指针解引用
	if d == nil {
		return "-"
	}

	if v, ok := d[k]; ok {
		return v
	}

	return "-"
}

var IsMap Dict = map[int8]string{
	global.No:  "否",
	global.Yes: "是",
}

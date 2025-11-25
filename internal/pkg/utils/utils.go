package utils

import (
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/samber/lo"
)

// CalculateChanges 计算差集 （一次性获取删除、新增和剩余列表）
// 计算交集
// 合并差集和交集
// 示例：
//
//	existingIds := []int{1, 2, 3, 4, 5}
//	ids := []int{2, 3, 6, 7}
//	toDelete, toAdd, remainingList := CalculateChanges(existingIds, ids)
//	fmt.Println("toDelete:", toDelete)
//	fmt.Println("toAdd:", toAdd)
//	fmt.Println("remainingList:", remainingList)
//
// 输出：
// toDelete: [1 4 5]
// toAdd: [6 7]
// remainingList: [2 3 6 7]
func CalculateChanges[T comparable](existingIds, ids []T) (toDelete, toAdd, remainingList []T) {
	// 2. 计算差集（一次性获取删除和新增列表）
	toDelete, toAdd = lo.Difference(existingIds, lo.Uniq(ids))

	// 2. 计算交集
	intersection := lo.Intersect(ids, existingIds)

	// 3. 合并差集和交集
	remainingList = lo.Union(intersection, toAdd)
	return
}

// RandString 生成随机字符串
func RandString(n int) string {
	letterBytes := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	var src = rand.NewSource(time.Now().UnixNano())

	const (
		letterIdxBits = 6
		letterIdxMask = 1<<letterIdxBits - 1
		letterIdxMax  = 63 / letterIdxBits
	)
	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

// TrimPrefixAndSuffixAND 去除字符串前后的 AND（不区分大小写，忽略多余空白）
func TrimPrefixAndSuffixAND(s string) string {
	s = strings.TrimSpace(s)

	// 正则匹配开头或结尾的 AND（忽略大小写和空白）
	re := regexp.MustCompile(`(?i)^(AND\s+)|(\s+AND)$`)
	for {
		trimmed := re.ReplaceAllString(s, "")
		if trimmed == s {
			break
		}
		s = strings.TrimSpace(trimmed)
	}

	return s
}

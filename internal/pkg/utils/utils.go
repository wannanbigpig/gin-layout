package utils

import (
	"crypto/rand"
	"regexp"
	"strings"

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

// RandString 生成长度为 n 的随机字符串。使用 crypto/rand 保证并发安全且不可预测，
// 通过拒绝采样消除取模偏置，适用于 token、文件名等需要避免碰撞的场景。
func RandString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if n <= 0 {
		return ""
	}
	// 拒绝阈值：丢弃 >= maxByte 的字节，使剩余取值能被字符集长度整除，避免偏置
	maxByte := 256 - (256 % len(letterBytes))
	result := make([]byte, n)
	buf := make([]byte, n)
	for i := 0; i < n; {
		if _, err := rand.Read(buf); err != nil {
			panic("utils.RandString: crypto/rand unavailable: " + err.Error())
		}
		for _, c := range buf {
			if int(c) >= maxByte {
				continue
			}
			result[i] = letterBytes[int(c)%len(letterBytes)]
			i++
			if i == n {
				break
			}
		}
	}
	return string(result)
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

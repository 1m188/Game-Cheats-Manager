// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// arabicToRoman 将阿拉伯数字（1-3999）转换为罗马数字。
// 参考 Python download_base_thread.py:276-293。
// 0 会直接返回字符 "0"。
func arabicToRoman(n int) string {
	if n == 0 {
		return "0"
	}

	numeralMap := []struct {
		value  int
		symbol string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}

	var result strings.Builder
	remaining := n
	for _, nm := range numeralMap {
		for remaining >= nm.value {
			_, _ = result.WriteString(nm.symbol)
			remaining -= nm.value
		}
	}
	return result.String()
}

// Sanitize 对字符串进行标准化处理：数字转罗马数字、去除标点（保留 &）、去空白、转小写。
// 参考 Python download_base_thread.py:295-298。
func Sanitize(s string) string {
	if s == "" {
		return ""
	}

	// 步骤 1: 数字转罗马数字
	digitRe := regexp.MustCompile(`\d+`)
	replaced := digitRe.ReplaceAllStringFunc(s, func(m string) string {
		n, err := strconv.Atoi(m)
		if err != nil {
			return m
		}
		return arabicToRoman(n)
	})

	// 步骤 2: 去除标点符号（保留 &）
	// 等价于 Python string.punctuation.replace('&', '')
	// 使用 [^a-zA-Z0-9&] 显式排除下划线（\w 包含 _，但 Python 中 _ 属于标点）
	punctRe := regexp.MustCompile(`[^a-zA-Z0-9&]`)
	cleaned := punctRe.ReplaceAllString(replaced, "")

	// 步骤 3: 转小写
	return strings.ToLower(cleaned)
}

// levenshteinDistance 计算两个字符串之间的 Levenshtein（编辑）距离。
// 用作 fuzzysearch 匹配失败时的后备近似匹配。
func levenshteinDistance(a, b string) int {
	// 确保 a 是较短的字符串，优化内存使用
	if len(a) > len(b) {
		a, b = b, a
	}

	aLen := len(a)
	bLen := len(b)

	// 使用两个一维数组代替二维数组，O(min(m,n)) 空间
	prev := make([]int, aLen+1)
	curr := make([]int, aLen+1)

	for i := 0; i <= aLen; i++ {
		prev[i] = i
	}

	for j := 1; j <= bLen; j++ {
		curr[0] = j
		for i := 1; i <= aLen; i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min(
				prev[i]+1,      // 删除
				curr[i-1]+1,    // 插入
				prev[i-1]+cost, // 替换
			)
		}
		prev, curr = curr, prev
	}

	return prev[aLen]
}

// isFuzzyMatch 使用多层匹配策略判断关键词是否匹配目标游戏名。
// 逻辑：fuzzy.Match → strings.Contains → Levenshtein 后备。
// 等价于 Python fuzzywuzzy 的 fuzz.partial_ratio >= 80。
func isFuzzyMatch(sanitizedKeyword, sanitizedGameName string) bool {
	if fuzzy.Match(sanitizedKeyword, sanitizedGameName) {
		return true
	}

	// 后备 1: 子串包含检查
	if strings.Contains(sanitizedGameName, sanitizedKeyword) {
		return true
	}

	// 后备 2: Levenshtein 编辑距离阈值
	// 允许编辑距离不超过关键词长度 1/3 的近似匹配
	threshold := len(sanitizedKeyword) / 3
	if threshold < 1 {
		threshold = 1
	}
	return levenshteinDistance(sanitizedKeyword, sanitizedGameName) <= threshold
}

// matchQuality 计算关键词与游戏名的匹配质量分数，值越小匹配越精确。
// 0=全字匹配, 1=前缀匹配, 2=子串匹配, 3=fuzzy匹配, 4=Levenshtein后备。
func matchQuality(sanitizedKeyword, sanitizedGameName string) int {
	if sanitizedKeyword == sanitizedGameName {
		return 0
	}
	if strings.HasPrefix(sanitizedGameName, sanitizedKeyword) {
		return 1
	}
	if strings.Contains(sanitizedGameName, sanitizedKeyword) {
		return 2
	}
	if fuzzy.Match(sanitizedKeyword, sanitizedGameName) {
		return 3
	}
	return 4 // Levenshtein
}

// versionRe 从游戏名中提取版本号（v1.2.3 格式）。
var versionRe = regexp.MustCompile(`v(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

// parseVersion 从游戏名提取版本号用于排序比较。无版本号返回 [0]。
func parseVersion(name string) []int {
	m := versionRe.FindStringSubmatch(name)
	if m == nil {
		return []int{0}
	}
	var v []int
	for _, s := range m[1:] {
		if s == "" {
			break
		}
		n, _ := strconv.Atoi(s)
		v = append(v, n)
	}
	if len(v) == 0 {
		return []int{0}
	}
	return v
}

// compareVersion 比较两个版本号，返回 -1/0/1（类似 strings.Compare）。
func compareVersion(a, b string) int {
	va := parseVersion(a)
	vb := parseVersion(b)
	for i := 0; i < len(va) && i < len(vb); i++ {
		if va[i] != vb[i] {
			return vb[i] - va[i] // 降序：新版本在前
		}
	}
	return len(vb) - len(va) // 版本号更具体的排在前面
}

// SearchTrainers 在 archive 和 main 索引中搜索匹配关键词的修改器。
// 以 URL 为去重键（同款游戏的不同版本保留为独立条目），main 来源优先。
// 排序：匹配质量 → 版本号降序 → GameName 字母序。
func SearchTrainers(archive, main []Trainer, keyword string) []SearchResult {
	if keyword == "" {
		return nil
	}

	sanitizedKeyword := Sanitize(keyword)

	// 前置条件: 关键词和游戏名清洗后至少需要 2 个字符
	if len(sanitizedKeyword) < 2 {
		return nil
	}

	// 使用 map 去重，key = trainer.URL（不同版本有不同 URL，全部保留）
	dedup := make(map[string]Trainer)
	quality := make(map[string]int)

	// 先处理 archive，再处理 main（同 URL 时 main 覆盖 archive）
	for _, t := range archive {
		key := Sanitize(t.GameName)
		if len(key) < 2 {
			continue
		}
		if _, exists := dedup[t.URL]; !exists && isFuzzyMatch(sanitizedKeyword, key) {
			dedup[t.URL] = t
			quality[t.URL] = matchQuality(sanitizedKeyword, key)
		}
	}
	for _, t := range main {
		key := Sanitize(t.GameName)
		if len(key) < 2 {
			continue
		}
		if isFuzzyMatch(sanitizedKeyword, key) {
			dedup[t.URL] = t // main 覆盖同 URL 的 archive 条目
			quality[t.URL] = matchQuality(sanitizedKeyword, key)
		}
	}

	// 收集结果并设置展示名
	results := make([]SearchResult, 0, len(dedup))
	for urlKey, trainer := range dedup {
		trainer.TrainerName = "[FL] " + trainer.GameName + " Trainer"
		results = append(results, trainer)
		_ = urlKey
	}

	// 排序：匹配质量 → 版本号降序（新在前）→ GameName 字母序
	slices.SortFunc(results, func(a, b SearchResult) int {
		qa := quality[a.URL]
		qb := quality[b.URL]
		if qa != qb {
			return qa - qb
		}
		if v := compareVersion(a.GameName, b.GameName); v != 0 {
			return v
		}
		return strings.Compare(
			strings.ToLower(a.GameName),
			strings.ToLower(b.GameName),
		)
	})

	return results
}

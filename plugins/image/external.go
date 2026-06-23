package image

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// resolveURL 把 pick 取到的（可能是相对路径的）地址补成可访问的完整 URL。
// 已是 http(s) 绝对地址则原样返回；否则 base 非空时拼到 base 后面。
func resolveURL(base, u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if base == "" {
		return u
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(u, "/")
}

// pathStep 是 pick 路径的一步：取对象字段，或取数组下标（固定/随机）。
type pathStep struct {
	field    string // 非空：取对象字段
	isIndex  bool   // true：取数组下标
	index    int    // 固定下标
	isRandom bool   // 随机下标（[*] / [random]）
}

// parsePickPath 解析 pick 路径，支持 a.b 取字段、[N] 取第 N 个、[*]/[random] 随机一个。
// 例：data[*].url、data[0].url、list.items[2].src。
func parsePickPath(path string) ([]pathStep, error) {
	var steps []pathStep
	for i := 0; i < len(path); {
		switch path[i] {
		case '.':
			i++
		case '[':
			j := strings.IndexByte(path[i:], ']')
			if j < 0 {
				return nil, fmt.Errorf("路径 %q 的 [ 未闭合", path)
			}
			inner := path[i+1 : i+j]
			i += j + 1
			if inner == "*" || inner == "random" {
				steps = append(steps, pathStep{isIndex: true, isRandom: true})
				continue
			}
			n, err := strconv.Atoi(inner)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("路径 %q 含非法下标 [%s]（应为 数字 / * / random）", path, inner)
			}
			steps = append(steps, pathStep{isIndex: true, index: n})
		default:
			k := i
			for k < len(path) && path[k] != '.' && path[k] != '[' {
				k++
			}
			steps = append(steps, pathStep{field: path[i:k]})
			i = k
		}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("路径为空")
	}
	return steps, nil
}

// ExtractImageURL 按 pick 路径从 JSON 取图片 URL。数组必须用显式下标
// （[N] 固定 / [*] 或 [random] 随机），不写下标命中数组即报错，不再隐式随机。
// path 必须非空（path 为空表示「响应本身即图」，由调用方在外层处理）。
func ExtractImageURL(jsonBody []byte, path string) (string, error) {
	steps, err := parsePickPath(path)
	if err != nil {
		return "", err
	}
	var cur any
	if err := json.Unmarshal(jsonBody, &cur); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %w", err)
	}
	for _, s := range steps {
		if s.isIndex {
			arr, ok := cur.([]any)
			if !ok {
				return "", fmt.Errorf("路径 %q：下标处不是数组", path)
			}
			if len(arr) == 0 {
				return "", fmt.Errorf("路径 %q：数组为空", path)
			}
			idx := s.index
			if s.isRandom {
				idx = rand.Intn(len(arr))
			}
			if idx >= len(arr) {
				return "", fmt.Errorf("路径 %q：下标 %d 越界（共 %d 个）", path, idx, len(arr))
			}
			cur = arr[idx]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("路径 %q 在 %q 处不是对象（数组请用 [N]/[*]）", path, s.field)
		}
		v, ok := m[s.field]
		if !ok {
			return "", fmt.Errorf("路径 %q 缺少键 %q", path, s.field)
		}
		cur = v
	}
	str, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("路径 %q 结果不是字符串", path)
	}
	return str, nil
}

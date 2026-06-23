package image

import (
	"encoding/json"
	"fmt"
	"math/rand"
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

// ExtractImageURL 按点路径从 JSON 取图片 URL；遇数组自动随机抽一个。
// path 必须非空（path 为空表示「响应本身即图」，由调用方在外层处理）。
func ExtractImageURL(jsonBody []byte, path string) (string, error) {
	var root any
	if err := json.Unmarshal(jsonBody, &root); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %w", err)
	}
	cur := root
	for _, key := range strings.Split(path, ".") {
		if key == "" {
			continue
		}
		if arr, ok := cur.([]any); ok {
			if len(arr) == 0 {
				return "", fmt.Errorf("路径 %q 处数组为空", path)
			}
			cur = arr[rand.Intn(len(arr))]
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("路径 %q 在 %q 处不是对象", path, key)
		}
		v, ok := m[key]
		if !ok {
			return "", fmt.Errorf("路径 %q 缺少键 %q", path, key)
		}
		cur = v
	}
	if arr, ok := cur.([]any); ok {
		if len(arr) == 0 {
			return "", fmt.Errorf("路径 %q 结果数组为空", path)
		}
		cur = arr[rand.Intn(len(arr))]
	}
	s, ok := cur.(string)
	if !ok {
		return "", fmt.Errorf("路径 %q 结果不是字符串", path)
	}
	return s, nil
}

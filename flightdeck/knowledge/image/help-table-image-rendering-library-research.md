# 帮助图片使用两遍表格布局

帮助详情图保持纯 Go 渲染：第一遍逐 rune 测量左右单元格并计算每行高度，第二遍绘制表头、交替背景、分隔线和文字。中文连续文本不能依赖按空格分词的换行器。

实现使用 `golang.org/x/image/font/opentype` 加载项目字体，以标准库 `image/png` 无损输出；结构化行、独立列换行和自动行高集中在 [`plugins/system/help_image.go`](../../../plugins/system/help_image.go)，回归测试在 [`plugins/system/help_image_test.go`](../../../plugins/system/help_image_test.go)。这套布局只负责“命令 + 说明”的两列表格，不扩散到其他图片业务。

测试依赖可用的中文字体。若开发机没有项目预期字体，字体加载测试失败属于环境前置条件，不应通过放宽像素布局断言掩盖。

只有当帮助内容需要多字体、图标、复杂徽章、响应式主题，且部署镜像已经稳定携带 Chromium 时，才重新评估 HTML/CSS 截图；当前需求无需新增浏览器或通用表格图片库。

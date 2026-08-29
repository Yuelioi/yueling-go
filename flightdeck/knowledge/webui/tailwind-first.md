# WebUI 页面样式以 Tailwind utilities 为首选

新增或修改 Vue 页面时，布局、间距、尺寸、颜色、边框、排版、交互状态和响应式规则默认直接写成 Tailwind utilities，让样式与使用位置保持同一局部。

`webui/src/assets/main.css` 只承载全局设计 token、基础元素规则和被多个页面稳定复用的共享模块。单个页面或单个模块专用的选择器不能加入全局样式表；已有共享类可以继续复用，但不要为了新页面扩充一组页面名前缀选择器。

D3 坐标、运行时尺寸等动态值可以使用 Vue `:style`。复杂但静态的可视化纹理优先用 Tailwind arbitrary values；确实不适合 utility 时放在所属模块的 scoped 样式中，并保持范围最小。

完成前运行 `git diff <基线> -- webui/src/assets/main.css`：如果一个单页功能让全局表增长，应先把样式迁回页面或提取为拥有小接口的复用模块。

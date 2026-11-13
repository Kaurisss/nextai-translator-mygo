# 原生 UI 组件对照

核对版本：MyGo v0.8.3。通过 Go 模块版本列表确认该版本为当前最新发布，并阅读模块缓存中的 `docs/ui/README.md`、相关组件文档和构造函数清单。

| 界面需要 | 官方组件 | 选择理由 |
| --- | --- | --- |
| 功能导航 | `ui.Toolbar`、`ui.Button`、`PopoverMenuButton` | 底栏设置与历史入口，更多菜单进入动作管理，输入工具栏进入生词本 |
| 操作组 | `ui.Toolbar`、`ui.Button`、`ui.PrimaryButton` | 键盘漫游与溢出菜单 |
| 收藏状态 | `ui.Toggle` | 使用官方的按下状态和可访问语义；置顶在更多菜单中使用 Checked 状态 |
| 可搜索的语言选择 | `ui.ComboboxBase` | 保留官方输入、搜索、方向键选择；给下拉箭头保留独立的尾侧空间 |
| 动作、模型服务、外观选择 | `ui.Select` | 受限选项的原生下拉行为 |
| 原文和提示词 | `ui.TextArea` | 中文输入法、选择、撤销、原生剪贴板和滚动 |
| 设置 | 原生输入 + `Text.LabelledBy` | 固定标签列宽，标签统一左对齐；标签点击聚焦与密码字段保持原生行为 |
| 动作的高级选项 | `ui.Collapsible`、`ui.Checkbox` | 折叠次要配置，思考开关与表单一起保存 |
| 历史和生词 | `ui.List`、`ui.SearchField` | 虚拟化行与搜索清除行为 |
| 上下文本区域 | `ui.SplitVerticalPercent` | 默认输入区占 40%，包含独立提交行与工具行，官方分割条支持拖动和键盘调整 |
| 富文本结果 | `ui/markdown.View`、`Stream` | 流式解析、选择复制、代码、表格和数学 |
| 结果、表单滚动 | `ui.Scroll` | 触控板、滚轮、滚动条和键盘滚动 |
| 等待状态 | `ui.Spinner` | 标准不确定进度 |
| 操作反馈及撤销 | `Context.Toast`、`ToastAction` | 短暂反馈和可聚焦的撤销操作 |
| 图标操作 | `ui.Button`、`ui.Icon` | 官方按钮配合 SVG；所有按钮有名称和 Tooltip；图标路径只解析一次 |
| 隐藏标题栏 | `TitleBarHidden`、`Context.TitleBar`、`DragWindow` | 系统最小化、最大化、关闭按钮覆盖在内容上，保留系统边框、缩放和 Snap Layouts |
| 页面排版 | `ui.Row`、`ui.Column`、`ui.Text`、`ui.Spacer` | 只用于排版、标题、计数，不替代上述控件 |

`iconButton` 只组合官方 Button 与 Icon，统一尺寸、可访问名称和提示。应用图标复用源项目，操作图标来自 Material Design Icons，与源项目使用的 Material 图标语义一致；许可见 `resources/LICENSE-icons.txt`。

两处官方样式无法满足当前要求的组合已集中封装：`languagePicker` 使用 ComboboxBase，解决 v0.8.3 Combobox 内部输入区固定最小宽度 100 DIP、在窄语言框中将箭头挤到边框的问题；`settingsField` 使用 Text.LabelledBy 和原生输入控件，解决 Form 固定右对齐标签且无公开左对齐选项的问题。没有重写文本编辑、键盘选择、密码隐藏或焦点行为。语言选择器的箭头区域保持 8 DIP 尾侧留白；设置页标签列为 6 rem，字段列间距 12 DIP，说明文字和保存按钮与输入框起点一致。

当前界面以用户提供的第二张 NextAI Translator 截图为基准。截图按 Windows 150% 缩放估算，图中的 814×1318 物理像素约对应 543×879 DIP。主窗口默认 560×900 DIP，最小 500×560 DIP，标题行 32 DIP，底栏 40 DIP，左右正文边距 20 DIP，输入区约占正文 40%。这些是截图比例估计，并非 Figma 精确数值。独立 StateKey 避免原先宽窗尺寸覆盖新布局。

布局为细标题行、语言与动作工具栏、输入区及小工具、白底结果区、设置与历史底栏。移除侧栏、重复的原文/结果标题和紧凑切换。翻译、润色、总结、分析显示为 SVG 按钮，选中动作显示图标和名称；自定义动作被选中时进入第四个槽位。更多菜单保留所有动作、粘贴、截图、置顶、生词本和动作管理。子页通过内容区自己的标题行返回按钮或 Escape 回到翻译，应用标题栏始终不变。

颜色继承系统主题，强调色改用正文色和背景色，匹配参考图的黑白激活态。字号使用系统正文、12 DIP 标题/底栏/计数和子页 1.4 rem 标题。系统标题栏按钮的主题通过 `SetWindowButtonsTheme` 与内容同步。没有手绘窗口控制按钮，也未设置 Frameless 或 HideWindowButtons。

顶部动作只改变选择，不发起翻译。输入框非空时在其下方显示独立一行：字符数、Enter/Shift+Enter 提示、火箭 SVG 提交按钮；请求进行时同一位置变为停止按钮。文本控件的 OnTextKey 处理普通 Enter，Shift+Enter 留给原生编辑器插入换行，保留窗口级 Ctrl/Cmd+Enter。测试分别验证切换动作无请求、提交位置、Enter 提交和 Shift+Enter 换行。

控件状态保留在应用结构中；页面容器和输入均有稳定身份。视图只构建界面与处理交互，网络与系统操作运行在工作 goroutine，通过 `Window.Update` 回到 UI 线程。没有保存 `Element` 或 `Context`。翻译和语音任务使用取消与代次号丢弃过时回调。

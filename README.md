# NextAI Translator Native

基于 [nextai-translator](https://github.com/nextai-translator/nextai-translator) 使用 MyGo v0.8.3 和 Go 重构的原生桌面版。窗口内容由 `github.com/egoist/mygo/ui` 绘制，不启动 WebView，不需要 React、Node.js 或前端构建。

## 运行与构建

需要 Go 1.27.3 或支持自动下载工具链的 Go 安装。

```powershell
go run .
```

Windows 可执行文件：

```powershell
go tool mygo build -platform windows/amd64 -skip-installer
```

使用 MyGo 打包，应用信息位于 `mygo.json`，生成文件位于 `build/windows-amd64/`。该命令会将源项目图标、版本信息及 DPI manifest 写入 EXE，并使用 GUI 子系统。普通 `go build` 适合源码验证，不会自动嵌入这些 Windows 资源。窗口的任务栏图标也通过 `Window.SetIcon` 显式设置。

使用 Inno Setup 6 生成 Windows 安装包：

```powershell
./scripts/build-installer.ps1
```

脚本重新构建应用，从 `mygo.json` 读取版本，输出 `build/installer/nextai-translator-native-setup-<版本>-x64.exe`。编译器不在默认路径时使用 `-ISCC '编译器路径'`。安装包提供简体中文和英文界面，默认安装到当前用户的 `%LOCALAPPDATA%\Programs\NextAI Translator Native`，无需管理员权限，创建开始菜单入口，可选桌面快捷方式，并注册卸载。卸载保留用户数据、模型缓存和系统凭据。

首次启动后进入设置，选择服务，填写 API Key 和模型，保存设置。OpenAI 兼容服务填写完整的聊天接口地址；Azure 填写含部署名和 `api-version` 的完整请求地址；Gemini 填写 `https://generativelanguage.googleapis.com/v1beta` 一类基础地址。模型名称可以直接编辑，默认模型仅作为初始配置。

## 已实现的桌面功能

- 翻译、润色、总结、语法分析、代码解释、生词助记故事。
- 自定义提示词动作的创建、编辑、删除，支持原版 `${sourceLang}`、`${targetLang}`、`${text}`，兼容 `{{source}}` 和 `{{target}}`，可为动作指定模型服务、模型、输出格式与思考强度。
- 67 种语言选项、自动检测提示、语言交换、可搜索的语言选择器。
- 流式响应、取消请求、错误反馈；旧请求的回调不会覆盖新内容。
- 顶部动作按钮只切换功能类型；输入框下方右侧的独立提交按钮发送请求。Enter 提交，Shift+Enter 换行，同时保留 Ctrl/Cmd+Enter；输入法组合输入由原生文本控件处理。
- 原生 Markdown、代码块、表格、数学公式和可选择的结果文本。
- 历史记录、搜索、收藏筛选、恢复原文与译文、删除撤销。
- 生词收藏、去重、释义、复习计数、助记故事。
- CSV 导出历史和生词、JSON 备份；导入备份时合并历史与生词，不覆盖当前模型配置。
- 系统主题跟随、浅色与深色外观、上下排列的输入与结果区、可拖动调整输入区高度、窗口置顶。
- 隐藏系统标题栏，原生最小化、最大化、关闭按钮嵌入内容顶部；保留窗口拖动、缩放和 Windows 贴靠行为。
- 与原项目一致的单列布局，SVG 操作按钮，底部设置和历史入口；生词本、动作管理等功能通过工具栏和更多菜单访问。
- 系统托盘、单实例、关闭窗口隐藏到托盘、菜单退出。
- 可配置的全局取词快捷键，默认 `Ctrl+Shift+T`，macOS 为 `Cmd+Shift+T`。
- 图片识别、区域截图后识别并翻译、系统语音朗读及停止。
- Windows 下将结果写回全局取词时记住的窗口。

## 设置

设置按原版分为五个页签，表单滚动，返回与保存固定在底部。未保存的编辑保留在草稿中，保存失败不会替换已应用的配置。

| 页签 | 功能 |
| --- | --- |
| 通用 | 界面语言、服务提供商、多 API 密钥、模型列表刷新与手动模型、API URL 和路径、全局思考、Azure Token 上限、Ollama 驻留时间、默认动作与目标语言、外观、备份 |
| 代理 | HTTP、HTTPS、SOCKS5、服务器与端口、认证、绕过列表、连接测试 |
| TTS | 系统语音和本地 MeloTTS/Kokoro、语速、音量、按语言和提供者保存音色、试听、停止、朗读输入框选中词语 |
| 写作 | 目标语言、写作快捷键、换行按键；翻译完成后检查目标窗口与原文，再替换输入内容 |
| 快捷键 | 取词、显示窗口、截图 OCR、快捷翻译；点击录制、清除和冲突检测 |

默认动作与目标语言用于下次启动；翻译页当前语言单独保留。快捷键注册失败会恢复原有注册。快捷翻译通过系统托盘的原生浮动窗口显示，不抢占其他应用的键盘焦点；Windows 读取指针下的 UI Automation 文本或标准文本控件，密码输入框不读取。macOS/Linux 使用所选文本。

界面语言复用原版的简体中文、繁体中文、英语、日语、韩语、泰语、土耳其语和巴西葡萄牙语资源；新增原生文案在其他语言缺少译文时使用英文。更新原版翻译后可执行 `node scripts/ui-locales.mjs` 重新生成嵌入资源。

本地 TTS 在首次使用时下载与原版一致的 MeloTTS/Kokoro 模型和 sherpa-onnx 1.13.4 运行程序，均校验 SHA-256，缓存于用户数据目录的 `tts/`。下载、解压和合成在后台运行，可通过停止朗读取消；不需要 Python、CGO 或额外手动配置。本地语音支持英语和中文，其他语种使用系统语音。Windows amd64、Linux amd64 与 macOS 的运行时下载已接入。

## 服务协议

| 协议 | 服务 |
| --- | --- |
| OpenAI Chat Completions | OpenAI、DeepSeek、Ollama、OpenRouter、LiteLLM、Groq、Cerebras、Moonshot、Kimi、MiniMax、ChatGLM、TeamoRouter |
| Azure Chat Completions | Azure，使用 `api-key` 请求头 |
| Anthropic Messages | Claude |
| Gemini GenerateContent | Gemini |
| Cohere Chat v2 | Cohere |

未启用自定义代理时，HTTP 客户端使用 MyGo `netproxy.Transport()`，遵循系统代理和代理环境变量；启用后，翻译、模型列表和语音资源下载使用已保存的代理及绕过列表。单次请求最长五分钟，不自动重试翻译请求。

提示词生成已迁移原版 `src/common/translate.ts` 的分支规则，包括短文本、单词词典、选中词语境、写作模式、润色、总结、分析、代码解释和助记故事。各协议使用与原版一致的合并 user 消息，并迁移模型默认参数与思考配置。旧原生版中未经修改的默认提示词会自动升级；自定义提示词、模型覆盖和已有数据保留。编辑内置动作的提示词后，该动作转为自定义动作。

词典模式通过系统 ICU 对齐 `Intl.Segmenter` 的词边界。Windows 使用系统 `icu.dll`，macOS 使用系统 `libicucore`，Linux 需要安装运行时 ICU 库（`libicuuc.so.*`）。

## 数据与系统能力

数据位于 MyGo `App.Path(PathUserData)` 下的 `data.json`。文件保存使用同目录临时文件替换；损坏数据会报告错误，不会自动覆盖。API Key 和代理密码单独保存在系统凭据库，服务名为 `xyz.nextai.translator.native`，JSON 和 CSV 均不包含这些凭据。旧原生配置加载时补充新增设置的默认值，保留用户数据和已有模型配置。

| 平台 | OCR | 区域截图 | 朗读 | 全局取词与写回 |
| --- | --- | --- | --- | --- |
| Windows | Windows.Media.Ocr，使用已安装的 OCR 语言包 | 系统截图工具 | System.Speech | Win32 复制、记住目标窗口、激活后粘贴 |
| macOS | 需要 `tesseract` 与 `eng`、`chi_sim` 语言数据 | `screencapture -i` | `say` | AppleScript 复制；写回使用手动粘贴 |
| Linux | 需要 `tesseract` 与语言数据 | 需要 `gnome-screenshot` | 需要 `espeak` | 需要 X11 `xdotool`；写回使用手动粘贴 |

Windows 的 PowerShell 脚本随二进制嵌入。文本通过标准输入传给语音进程，图片路径通过环境变量传递，不拼接进脚本。截图取消会在超时后回到主窗口并显示提示。

## 与现有版本的差异

此模块实现原生桌面的主要工作流，以下功能尚未迁移：ChatGPT/Kimi 网页登录会话接入、鼠标划词悬浮图标、逐词悬停词典、自动更新、原版 IndexedDB 和设置的自动迁移。原版当前隐藏的 Edge TTS 服务不提供选项。

浏览器扩展和用户脚本继续使用原来的 TypeScript 实现。自动更新需要为原生安装包配置独立的签名密钥和发布源后再接入；不会复用 Tauri 的更新包。

## 验证

```powershell
go test ./...
go vet ./...
go tool mygo vet .
$env:CGO_ENABLED = '0'
go test ./...
go build ./...
```

测试覆盖五类请求协议与鉴权、思考配置、SSE 多行和尾事件、HTTP 与协议错误、响应中断、取消、持久化替换、损坏数据保护、页面导航、最小窗口、SVG 按钮名称、上下文本区几何、更多菜单、Escape 返回、复制、语言交换、生词复习、自定义动作、翻译与历史恢复，以及取消后的过时回调。

提示词差分测试使用原版 TypeScript 实际执行生成的 111 组提示词、21 组分词、16 组模型参数和 20 组 Gemini 思考配置样本，并验证各协议的消息内容、实际 HTTP 请求和默认动作迁移。重新生成样本需要 Node.js（支持 `stripTypeScriptTypes`）、`common-tags@1.8.2` 和 `iso-639-1@3.0.1`，与根目录锁文件保持一致。依赖可安装在仓库外的临时目录：

```powershell
node scripts/prompt-reference.mjs --dependencies '包含 node_modules 的目录'
go test ./internal/core
```

生成器同时更新嵌入的语言元数据和字符表；更新原版规则后应重新生成并运行测试。

Windows OCR 集成验证：

```powershell
$env:NEXTAI_OCR_TEST_IMAGE = (Resolve-Path '你的测试图片.png').Path
go test ./internal/desktop -run TestWindowsOCR -v
```

导出离屏截图时设置 `NEXTAI_SCREENSHOT_DIR` 为现有目录，运行 `go test ./internal/app -run 'Test.*RenderArtifact' -count=1`，包含五个设置页签。

设置测试覆盖代理认证与绕过、各协议模型列表、Gemini 生成能力及思考能力、参数生效、旧配置迁移、草稿隔离、语言切换与持久化值、页签导航、错误提示不移动保存栏、快捷键录制、写作取消、下载校验与解压路径，以及配置写入失败和凭据批次失败时的恢复。设置 `NEXTAI_SYSTEM_TTS_TEST=1` 可运行 Windows 系统音色与静音朗读验证；`NEXTAI_LOCAL_TTS_TEST_ROOT` 指向含已校验运行时和模型的目录时，可验证 MeloTTS/Kokoro 的实际合成及静音播放。

自动化仅包含在 `e2e` 构建中：

```powershell
go build -tags e2e -o build/nextai-translator-e2e.exe .
$env:MYGO_AUTOMATION = "$env:LOCALAPPDATA/Temp/opencode/nextai-native.sock"
./build/nextai-translator-e2e.exe
```

另一个终端可用 `go run github.com/egoist/mygo/plugins/automation/cmd/mygo-automation skills` 查看匹配版本的操作方式。

已在 Windows 验证编译、无 CGO 构建、单元与原生 UI 测试、真实 GPU 窗口启动、输入、导航、中英文 OCR、凭据库写入/读取/删除、系统及 MeloTTS/Kokoro 语音合成与静音播放，并在独立文本控件验证高 DPI 指针取词及中文、Emoji、多行写入。全局快捷键到浮窗、截图工具和真实应用的完整交互仍需桌面联调；线上模型调用需要用户配置有效 Key。macOS/Linux 尚未实机验证。

## 开源协议与说明

- 本项目基于 [nextai-translator/nextai-translator](https://github.com/nextai-translator/nextai-translator) 重构，继续遵循 [AGPL-3.0](./LICENSE) 协议开源。
- 界面框架使用 [github.com/egoist/mygo](https://github.com/egoist/mygo)（MIT License）；界面图标来自 Pictogrammers Material Design Icons，许可说明见 [resources/LICENSE-icons.txt](./resources/LICENSE-icons.txt)。
- `scripts/ui-locales.mjs` 与 `scripts/prompt-reference.mjs` 仅用于从原仓库 `src/common/` 同步多语言与提示词参考样本；日常构建、测试与打包不依赖上层目录。

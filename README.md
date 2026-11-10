# Codex Image 2.5 Skill（鹊桥赞助支持）

让 Codex 通过你配置的 OpenAI 兼容 API 生成、编辑图片，也可以生成视频。生图默认 **GPT Image 2.5（`gpt-image-2.5`）+ 1K**（成本更低）；视频默认 **`grok-imagine-video` + 1K**。用户可以问「现在支持哪些模型」，也可以直接说「用 2.6」「G2.6」「这张 4K」「生成视频」，不必为每个新模型再更新 Skill。

日常直接说「用 Image 2」「换成 2.6」「用 Grok」「做个视频」即可。同一对话保留已经选择的模型和分辨率，换模型、换画质都不用改配置或重启。

- 仓库：**codex-image2-5-skill**
- 显示名称：**Codex Image 2.5**
- 技能名称及安装目录：**codex-image2-5**
- 调用方式：`$codex-image2-5`
- 支持文生图、单图/多图编辑、PNG Mask、JSONL 批量生图、文生视频；可查询当前网关有哪些模型。
- 内置 Windows x64/ARM64、macOS Intel/Apple Silicon 程序，无需安装 Python、Node.js 或 Go。

## 安装或升级

把下面这段发给 Codex：

```text
请帮我安装或更新 Codex Image 2.5：
https://github.com/daifei-0/codex-image2-5-skill
技能位于仓库内 codex-image2-5 目录。
如果已装旧版 codex-image2，请先备份旧版到技能目录之外，再替换为新版，避免两个版本同时启用。
保留已有 CODEX_API_URL 和 CODEX_API_KEY，不要打印密钥。
```

旧版技能名称是 `codex-image2`。仅改 GitHub 仓库名不会更新已经安装的文件，需更新技能后使用 `$codex-image2-5`。已有 API 地址和密钥可继续使用；默认模型要求令牌有 `gpt-image-2.5` 权限，改用 2.6 或其他模型时令牌也要有对应权限，价格以所用平台为准。

手动安装（首次安装）：

Windows PowerShell：

```powershell
git clone https://github.com/daifei-0/codex-image2-5-skill.git
Copy-Item codex-image2-5-skill\codex-image2-5 "$HOME\.codex\skills\codex-image2-5" -Recurse
```

macOS：

```bash
git clone https://github.com/daifei-0/codex-image2-5-skill.git
cp -R codex-image2-5-skill/codex-image2-5 ~/.codex/skills/codex-image2-5
chmod +x ~/.codex/skills/codex-image2-5/bin/codex-image2-5-darwin-*
```

## 配置 API

程序读取两个环境变量：

- `CODEX_API_URL`：API 根地址或以 `/v1` 结尾的地址。
- `CODEX_API_KEY`：你自己的 API 密钥，只在本机安全设置，不发到聊天、截图或仓库。

使用鹊桥时，地址填写 `https://cdn.5202828.xyz/v1`。在控制台创建具有目标生图模型权限的令牌。

Windows 可在系统「编辑账户的环境变量」中设置以上两项。macOS 可在自己的 shell 环境中设置；从桌面启动的客户端需要能继承该环境。不要把密钥写进 SKILL.md。

**如果正在运行的客户端没有读取新设置的环境变量，完全退出后重新打开。日常切换模型、分辨率不需要重启。** 已有配置无需重设。

## 白话文切换模型

明确指定的模型优先；没指定的选项沿用本对话已有选择。新对话生图默认 Image 2.5 + 1K，视频默认 `grok-imagine-video` + 1K。

问「支持哪些模型 / 现在能用什么」时，程序会向网关拉取当前模型列表，再按你的说法自动对应，不必再指定一次完整 id。

| 你说 | 实际模型 |
| --- | --- |
| 新对话不指定模型 / GPT / GPT Image | `gpt-image-2.5`（默认，更便宜） |
| Image 2 / image2 / GPT Image 2 / img2 / Image 二 | `gpt-image-2` |
| Image 2.5 / 2.5 / 二点五 | `gpt-image-2.5` |
| Image 2.6 / 2.6 / G2.6 / 以麦 G2.6 / 二点六 | `gpt-image-2.6` |
| Image 2.7 / 2.7 / 以后的版本号 | `gpt-image-2.7` 等，按你说的版本生成，不必等 Skill 更新 |
| 闪焰 / flare（未点版本） | `gpt-image-2.5-flare` |
| 2.6 闪焰 / Image 2.6 flare | `gpt-image-2.6-flare` |
| 日耀 / sunburst（未点版本） | `gpt-image-2.5-sunburst` |
| 最新 / latest | `chatgpt-image-latest` |
| Grok / Grok 2 / Grok 2.0 | `grok-imagine-image-2.0` |
| Grok 高质量 / Grok 质量版 / Grok quality | `grok-imagine-image-quality` |
| 点名 grok-imagine / Grok Imagine | `grok-imagine` |
| 生成视频 / 做个视频（未点模型） | `grok-imagine-video` |
| Grok 视频 / Grok video 1.5 / sora | `grok-imagine-video` / `grok-imagine-video-1.5` / `sora-2` |
| 网关上的完整模型名 | 原样传给 API |

模型别名不区分英文字母大小写，支持常见空格、连字符、下划线、全角字母数字和中文数字。CLI 的 `--model` 接收版本号、别名或网关 id；完整聊天句子由 Codex 提取模型与分辨率。说「2K」「4K」即可换分辨率，未指定时一律 1K。

**技能名称里的「2.5」只表示生图默认，不锁死模型。** 「用 Image 2」是 Image 2，「用 2.6」是 Image 2.6。只说「高质量一点」保留当前模型，提高其质量参数，不会自动跳到 Grok。画面文字、尺寸中的数字也不会被当成模型名。可用 `CODEX_IMAGE_DEFAULT_MODEL` / `CODEX_VIDEO_DEFAULT_MODEL` 改默认，不必改 Skill 文件。

连续对话示例：

```text
使用 $codex-image2-5，用 Image 2，2K，画一张海边日出。
这张改成 4K，模型不变。
换 Image 2.5 再画一张，分辨率不变。
换成 2.6。
改用 Grok，2K。
还是这个模型，画一张雪山。
```

以上分别使用 Image 2 + 2K、Image 2 + 4K、Image 2.5 + 4K、Image 2.6 + 4K、Grok + 2K、Grok + 2K。

## 1K / 2K / 4K

「2K」「2k」「2 K」「两K」「二K」都表示 2K。「一K」「四K」同理。只改分辨率时保留模型，只改模型时保留分辨率。「高清一点」没有指定像素档位，不擅自切换为收费可能更高的 4K。

| 选择 | GPT 生图 | Grok 生图 | 视频 |
| --- | --- | --- | --- |
| 1K（未指定时的默认） | 1024×1024 | 1k + 1024×1024 | 1280×720 |
| 2K | 2048×2048 | 2k + 2048×2048 | 1920×1080 |
| 4K | 3840×2160 | 请求上游最高档，不能保证原生 4K | 按 1080p 请求 |

以上是请求参数，最终像素以输出 `actual_size` 为准。部分上游可能降档或返回其他尺寸；程序会检查可识别图片的实际尺寸并对降档发出警告。Grok 当前4K适配仍发2k分辨率参数和较大size，历史上可返回约2816×1584，不能称为原生4K；GPT也不能仅凭请求参数保证3840×2160。

`--quality low|medium|high|auto` 是独立的质量参数，不等于 1K/2K/4K。Grok 模型的支持范围按现有适配处理。

## 改图

附上图片后说：

```text
使用 $codex-image2-5，用 Image 2.5，2K，修改这张图片：
只把背景替换成雪山，人物、服装、姿势和构图保持不变。
```

支持多张参考图、可选 PNG Mask；输出另存，不覆盖原图。

## CLI 用法

普通用户在对话中使用即可，以下用于自动化或调试。

| 系统 | 程序 |
| --- | --- |
| Windows x64 | `codex-image2-5/bin/codex-image2-5-windows-amd64.exe` |
| Windows ARM64 | `codex-image2-5/bin/codex-image2-5-windows-arm64.exe` |
| macOS Intel | `codex-image2-5/bin/codex-image2-5-darwin-amd64` |
| macOS Apple Silicon | `codex-image2-5/bin/codex-image2-5-darwin-arm64` |

默认生图：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" generate `
  --prompt "A tiny blue nebula inside a glass bottle" `
  --out "output/imagegen/nebula.png"
```

指定口语别名、仅校验参数，不调用 API：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" generate `
  --prompt "A seaside sunrise" `
  --model "Image 2.6" `
  --size "两K" `
  --dry-run
```

列出当前网关的生图和视频模型（新模型不必改 Skill）：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" models
```

生成视频（未指定模型时用 `grok-imagine-video`，未指定分辨率时用 1K）：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" video `
  --prompt "A red balloon drifting over a calm lake at sunset" `
  --size 1K `
  --out "output/videogen/balloon.mp4"
```

编辑：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" edit `
  --image "input.png" `
  --prompt "Replace only the background with a warm studio backdrop" `
  --model "Image 2.5" `
  --size 2K `
  --out "output/imagegen/edited.png"
```

批量格式见 [batch-format.md](codex-image2-5/references/batch-format.md)，完整技能说明见 [SKILL.md](codex-image2-5/SKILL.md)。

从源码构建 Windows x64（Go 标准库，无第三方依赖）：

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -trimpath -ldflags "-s -w" -o codex-image2-5/bin/codex-image2-5-windows-amd64.exe codex-image2-5/src/image_gen.go
```

## 鹊桥在线生图与教程

不安装技能也可以使用 [鹊桥在线生图](https://cdn.5202828.xyz/draw/)，网页支持的模型以页面为准。

![鹊桥在线生图](docs/images/queqiao-draw.jpg)

[鹊桥使用教程与常见问题](https://cdn.5202828.xyz/docs/faq.html)

## 常见问题

- **找不到新版技能**：更新后确认目录和名称都是 `codex-image2-5`，重新加载技能列表或开启新对话；旧安装不会随着仓库改名自动升级。
- **401 / 模型不可用**：检查令牌是否有效、分组及模型权限是否包含所选模型。程序不会偷偷换模型。
- **没有 API Key**：确认本机环境变量以及客户端进程是否已读取配置。
- **524 / 超时 / 429**：程序已有有限次数重试；仍失败时可减少批量并发、选1K或稍后再试，不会无限重试。
- **图片没有达到请求分辨率**：查看 `actual_size` 和 `warning`，按实际像素判断，不把请求档位当成生成结果。
- **视频**：说「生成视频」即可；未指定模型用 `grok-imagine-video`，未指定分辨率用 1K。点名 sora 或其他网关视频模型时按你说的调用。
- **其他中转站**：需兼容 `POST /v1/images/generations`、`POST /v1/images/edits`，并提供所选模型。

## 关于本仓库与许可

本项目作者为 [daifei-0](https://github.com/daifei-0)，由鹊桥赞助支持，采用 [MIT 许可证](LICENSE)。本版本支持查询当前网关模型、对话点名切换、1K/2K/4K，以及后续 GPT Image / 视频模型的规则适配。

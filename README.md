# Codex Image 2.5 Skill（鹊桥赞助支持）

仓库地址不变：https://github.com/daifei-0/codex-image2-5-skill

项目名称仍是 **Codex Image 2.5**。不指定模型时，生图继续使用更便宜的 **GPT Image 2.5（`gpt-image-2.5`）+ 1K**。

这次更新后，同一个技能可以直接使用新模型，不必换仓库、也不必换技能名：

- **GPT Image 2.6**：说「用 2.6」「G2.6」或「以麦 G2.6」，自动使用 `gpt-image-2.6`。
- **以后的新模型**：说「2.7」或其他版本号即可，对应 `gpt-image-{版本}`。网关新增模型后，不必再更新本 Skill。
- **查询当前模型**：说「现在支持哪些模型」，程序读取你的网关列表后回答。
- **分辨率**：说「1K」「2K」「4K」即可。没说时用 1K。
- **视频**：说「生成视频」。没指定视频模型时用 `grok-imagine-video`，没指定分辨率时用 1K。
- 原有的 Image 2、Image 2.5、闪焰、日耀和 Grok 生图仍然可用。

- 仓库：**codex-image2-5-skill**
- 显示名称：**Codex Image 2.5**
- 技能名称及安装目录：**codex-image2-5**
- 调用方式：`$codex-image2-5`
- 支持文生图、单图/多图编辑、PNG Mask、JSONL 批量生图、文生视频，以及查询当前模型。
- 内置 Windows x64/ARM64、macOS Intel/Apple Silicon 程序，无需安装 Python、Node.js 或 Go。

## 安装或升级

把下面这段发给 Codex：

```text
请帮我安装或更新 Codex Image 2.5：
https://github.com/daifei-0/codex-image2-5-skill
技能位于仓库内 codex-image2-5 目录。
如果已装旧版 codex-image2，请先停用旧版，再使用 codex-image2-5，避免两个版本同时启用。
保留已有 CODEX_API_URL 和 CODEX_API_KEY，不要打印密钥。
```

已经安装过的用户，把仓库更新到最新提交后，继续使用 `$codex-image2-5`。技能目录名没有改。令牌需要有所选模型的权限：默认是 `gpt-image-2.5`；说「用 2.6」、Grok 或视频时，令牌也要包含对应模型。价格以所用平台为准。

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

- `CODEX_API_URL`：API 根地址，或以 `/v1` 结尾的地址。
- `CODEX_API_KEY`：你自己的 API 密钥。只在本机设置，不要发到聊天、截图或仓库。

使用鹊桥时，地址填写 `https://cdn.5202828.xyz/v1`。在控制台创建具有目标模型权限的令牌。

Windows 可在系统「编辑账户的环境变量」中设置以上两项。macOS 可在自己的 shell 环境中设置；从桌面启动的客户端需要能继承该环境。不要把密钥写进 SKILL.md。

**如果正在运行的客户端没有读到新设置的环境变量，完全退出后重新打开。** 日常切换模型、分辨率不需要重启。已有配置无需重设。

## 怎么用

项目仍以 Image 2.5 为主。不点名模型时就是 2.5。要点新模型，直接说出版本。

```text
使用 $codex-image2-5，画一张海边日出。
这张改成 2K。
再改成 4K，模型不变。
换成 2.6，分辨率不变。
用 Grok，2K，再画一张雪山。
生成一段视频：红气球飘过湖面。
现在支持哪些模型？
```

上面依次是：Image 2.5 + 1K、Image 2.5 + 2K、Image 2.5 + 4K、Image 2.6 + 4K、Grok + 2K、默认视频 + 1K、读取网关模型列表。

| 你说 | 实际调用 |
| --- | --- |
| 不指定模型 / GPT / GPT Image / Codex Image 2.5 | `gpt-image-2.5`（默认，更便宜） |
| 用 2.6 / G2.6 / 以麦 G2.6 / 二点六 | `gpt-image-2.6` |
| Image 2 / image2 / GPT Image 2 | `gpt-image-2` |
| 2.7 / 以后的版本号 | `gpt-image-2.7` 等，按版本号生成，不必更新 Skill |
| 闪焰 / flare（未点版本） | `gpt-image-2.5-flare` |
| 2.6 闪焰 | `gpt-image-2.6-flare` |
| 日耀 / sunburst（未点版本） | `gpt-image-2.5-sunburst` |
| 最新 / latest | `chatgpt-image-latest` |
| Grok / Grok 2 | `grok-imagine-image-2.0` |
| Grok 高质量 | `grok-imagine-image-quality` |
| 点名 grok-imagine | `grok-imagine` |
| 生成视频（未点模型） | `grok-imagine-video` |
| Grok 视频 1.5 / sora | `grok-imagine-video-1.5` / `sora-2` |
| 网关上的完整模型名 | 原样传给 API |
| 2K / 4K | 只改分辨率，模型保持不变 |
| 没说分辨率 | 1K |

别名不区分大小写，支持空格、连字符、下划线、全角字符和中文数字。只说「高质量一点」会保留当前模型，并提高质量参数，不会自动改成 Grok，也不会擅自改成 4K。

可用 `CODEX_IMAGE_DEFAULT_MODEL` 和 `CODEX_VIDEO_DEFAULT_MODEL` 改默认模型，不必改 Skill 文件。

## 1K / 2K / 4K

「2K」「2k」「两K」「二K」都是 2K。「一K」「四K」同理。只改分辨率时保留模型，只改模型时保留分辨率。

| 选择 | GPT 生图 | Grok 生图 | 视频 |
| --- | --- | --- | --- |
| 1K（未指定时的默认） | 1024×1024 | 1k + 1024×1024 | 1280×720 |
| 2K | 2048×2048 | 2k + 2048×2048 | 1920×1080 |
| 4K | 3840×2160 | 请求上游最高档，不能保证原生 4K | 按 1080p 请求 |

最终像素以输出 `actual_size` 为准。上游可能降档。Grok 的 4K 请求历史上可能返回约 2816×1584，不能称为原生 4K。GPT 也不能仅凭请求参数保证 3840×2160。

`--quality low|medium|high|auto` 是质量参数，不等于 1K/2K/4K。

## 改图

附上图片后说：

```text
使用 $codex-image2-5，2K，修改这张图片：
只把背景替换成雪山，人物、服装、姿势和构图保持不变。
```

没另点模型时用 Image 2.5。要改用 2.6，加上「用 2.6」。支持多张参考图和可选 PNG Mask。输出另存，不覆盖原图。

## CLI 用法

普通用户在对话里说即可。下面用于自动化或调试。

| 系统 | 程序 |
| --- | --- |
| Windows x64 | `codex-image2-5/bin/codex-image2-5-windows-amd64.exe` |
| Windows ARM64 | `codex-image2-5/bin/codex-image2-5-windows-arm64.exe` |
| macOS Intel | `codex-image2-5/bin/codex-image2-5-darwin-amd64` |
| macOS Apple Silicon | `codex-image2-5/bin/codex-image2-5-darwin-arm64` |

默认生图（Image 2.5，1K）：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" generate `
  --prompt "A tiny blue nebula inside a glass bottle" `
  --out "output/imagegen/nebula.png"
```

指定 Image 2.6，并把分辨率说成「两K」，只校验、不调用 API：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" generate `
  --prompt "A seaside sunrise" `
  --model "2.6" `
  --size "两K" `
  --dry-run
```

查看当前网关有哪些生图和视频模型：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" models
```

生成视频。未指定模型和分辨率时，使用 `grok-imagine-video` 和 1K：

```powershell
& "codex-image2-5/bin/codex-image2-5-windows-amd64.exe" video `
  --prompt "A red balloon drifting over a calm lake at sunset" `
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

- **找不到更新**：目录和调用名仍是 `codex-image2-5`。更新仓库后重新加载技能，或开一个新对话。
- **401 / 模型不可用**：检查令牌、分组和模型权限是否包含你选的模型。默认需要 `gpt-image-2.5`；使用 2.6 或视频时需要对应权限。程序不会偷偷换模型。
- **没有 API Key**：确认本机环境变量，以及客户端进程是否已经读到配置。
- **524 / 超时 / 429**：程序会有限次重试。仍失败时减少批量并发、改用 1K，或稍后再试。
- **图片没有达到请求分辨率**：看 `actual_size` 和 `warning`，按实际像素判断。
- **视频**：说「生成视频」即可。未指定模型用 `grok-imagine-video`，未指定分辨率用 1K。点名其他视频模型时按你说的调用。
- **其他中转站**：需要兼容 `POST /v1/images/generations`、`POST /v1/images/edits`、`POST /v1/videos`，并提供所选模型。

## 关于本仓库与许可

本项目作者为 [daifei-0](https://github.com/daifei-0)，由鹊桥赞助支持，采用 [MIT 许可证](LICENSE)。项目名称保持 Codex Image 2.5。本版在此基础上支持 GPT Image 2.6、后续版本、当前模型查询和视频生成。

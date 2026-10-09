# 🪐 Orbit Agent

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://github.com/Motona-Sky/Orbit-agent/pulls)

[English](README_EN.md) | [简体中文](README.md)

[核心特性](#核心特性) · [快速开始](#快速开始) · [交互命令](#交互命令) · [工具与 Subagent](#内置工具与-subagent) · [MCP](#mcp) · [Skills](#skills) · [配置与数据](#配置与数据) · [开发](#开发与贡献)

Orbit 是使用 Go 编写的轻量级终端 AI Agent，以“模型请求 → 工具执行 → 结果回传”的循环完成任务，提供 TUI 对话、模型配置、Codex OAuth、MCP、Skills、会话恢复和简单的子 agent 委派。

> 本文档描述当前仓库源码的功能。项目仍在迭代中，发行包不一定包含最新源码改动；下载时请以对应版本的说明为准。

## 核心特性

- **OpenAI 兼容协议**：支持 Chat Completions 与 Responses，可配置自定义服务地址和模型。
- **Codex OAuth**：支持浏览器登录、导入本机 Codex 登录凭据，以及过期 Access Token 的刷新与保存。
- **内置工具**：执行命令、读取和修改文件、列出目录、搜索文件，以及维护全局长期记忆。
- **简单 Subagent**：主 agent 可同步委派只读子任务；子 agent 使用独立上下文，最终回答作为工具结果返回。
- **MCP 扩展**：支持本地 stdio、远程 Streamable HTTP 和 SSE 服务，提供服务状态查看与持久化启停开关。
- **Skills**：发现项目与用户目录下的 `SKILL.md`，通过斜杠命令指定技能和任务。
- **会话与上下文**：以 JSON 保存会话历史，支持选择和恢复历史会话；长上下文可自动压缩。
- **使用统计与调试**：记录每日 token 用量、缓存命中率，并可开启本地调试记录。
- **单二进制 TUI**：核心程序无需额外语言运行时，提供中文、英文界面与主题设置；第三方 MCP 服务仍需各自的运行环境。

### 模型支持边界

当前实际实现的协议为 `openai:completions`、`openai:responses` 和 `oauth:codex`。提供商配置界面的预设不等于已经实现原生协议适配：Anthropic Messages 尚未实现，Gemini 没有原生适配，Ollama 使用 OpenAI 兼容接口。

配置兼容服务时，Base URL 应包含服务端要求的 API 前缀，例如 `https://api.openai.com/v1`；程序会再追加 `/chat/completions` 或 `/responses`。当前主循环要求凭据非空，即使连接本地兼容服务也不能留空。

## 快速开始

### 1. 安装发行包

从 [Releases](https://github.com/Motona-Sky/Orbit-agent/releases) 下载对应系统与架构的安装包。当前打包脚本生成以下产物：

| 系统 | 架构 | 安装包命名 |
| :--- | :--- | :--- |
| Linux | amd64 / arm64 | `orbit-linux-<arch>.tar.gz` |
| Windows | amd64 / arm64 | `orbit-windows-<arch>.zip` |

Linux 示例，以 amd64 为例：

```bash
tar -xzf orbit-linux-amd64.tar.gz
bash ./orbit-linux-amd64/install.sh

# 如当前终端尚未加载更新后的 PATH
export PATH="$HOME/.local/bin:$PATH"
orbit
```

Linux 安装脚本将程序安装到 `~/.local/bin`。卸载可运行 `bash ./orbit-linux-amd64/install.sh uninstall`。

Windows 示例，在 PowerShell 中运行：

```powershell
Expand-Archive .\orbit-windows-amd64.zip -DestinationPath .
.\orbit-windows-amd64\install.ps1
```

Windows 安装目录为 `%USERPROFILE%\.orbit\bin`，脚本会更新用户 PATH。安装后重新打开终端，再运行 `orbit`。arm64 用户请将上述包名和目录名替换为对应架构。

### 2. 从源码构建

需要 Go **1.25 或更新版本**：

```bash
git clone https://github.com/Motona-Sky/Orbit-agent.git
cd Orbit-agent
go mod download
go build -o orbit ./cmd/orbit
./orbit
```

Windows 构建与启动：

```powershell
go build -o orbit.exe ./cmd/orbit
.\orbit.exe
```

也可以在仓库根目录执行 `go install ./cmd/orbit`，安装到 `GOBIN` 或 `GOPATH/bin`，并将相应目录加入 PATH。仓库的 `build.sh` 目前只生成上述 Linux / Windows 安装包，不生成 macOS 发行包；其他平台可按 Go 支持的目标从源码构建。

### 3. 初始化与启动

首次运行且没有配置时，Orbit 会进入语言、主题和提供商配置流程。也可主动运行：

```bash
orbit setup
```

选择 Codex 后可进行浏览器授权或导入本机 `~/.codex/auth.json` 中的登录凭据。登录或导入后，请在对话中使用 `/model` 选择模型；导入配置的模型字段初始可能为空。

常用启动方式：

```bash
orbit                 # 打开对话 TUI
orbit model           # 进入提供商配置界面
orbit -s              # 选择并恢复历史会话
orbit --session       # 与 -s 相同
orbit session         # 与 -s 相同
orbit --debug         # 启用调试记录并打开对话
```

以上命令假设 `orbit` 已在 PATH 中。本地构建时请改用 `./orbit`，Windows 使用 `.\orbit.exe`。

`--debug` 会在当前工作目录生成 `dbg-<session-id>` 调试文件。记录可能包含对话、模型请求和工具结果，分享前应检查并脱敏，不要提交到仓库。

## 交互命令

在聊天输入框中输入 `/` 可查看命令补全。

| 命令 | 当前行为 |
| :--- | :--- |
| `/model` | 为当前提供商选择或设置模型 |
| `/provider` | 配置或切换提供商 |
| `/effort` | 选择推理强度：`low`、`medium`、`high`、`xhigh`、`max`，默认 `high`；实际支持取决于服务端和模型 |
| `/skills` | 列出已发现的 Skills |
| `/skills <name> <task>` | 使用指定 Skill 执行任务，必须提供任务内容 |
| `/mcp` | 查看 MCP 服务状态，并启用或禁用服务 |
| `/new` | 开始新的会话 |
| `/clear` | 同样开始新的会话，不是仅清屏并保留当前上下文 |

注意：启动命令 `orbit model` 打开的是提供商配置界面，与对话中的 `/model` 模型选择界面不同。`/new` 和 `/clear` 不会作为保留上下文的清屏操作使用。

## 内置工具与 Subagent

### 工具列表

工具由模型按任务需要调用，并非直接输入的斜杠命令。名称区分大小写。

| 工具 | 用途 |
| :--- | :--- |
| `exec_command` | 执行系统命令 |
| `read_file` | 读取文件内容，支持行偏移和行数限制 |
| `ls` | 列出目录，可递归遍历 |
| `Grep` | 按文件名或内容搜索 |
| `update_file` | 修改文件内容 |
| `UpdateUserMemory` | 维护全局长期记忆，最多保存 10 条 |
| `subagent` | 同步委派只读检索或分析任务 |

默认审核模式会要求确认 `exec_command` 调用，但这不是完整的工具权限系统：文件修改、记忆更新和 MCP 调用不受该命令确认机制保护。工具运行在本机环境中，不是文件系统或进程沙箱。

### Subagent 子任务工具

`subagent` 唯一必填参数为 `task`：

```json
{
  "task": "在当前项目中查找工具注册入口，返回相关文件路径和实现说明。"
}
```

- 继承父 agent 当前使用的模型、认证、推理设置和系统约束，使用独立上下文。
- 不传入主会话历史，任务中应提供必要背景和文件路径。
- 只开放已启用的本地 `read_file`、`ls`、`Grep`；不修改文件、不执行命令、不调用 MCP，也不递归委派。
- 同步返回最终回答，父 agent 收到工具结果后继续处理；子请求 token 用量计入父会话。
- 每次最多 10 轮模型请求，运行超时为 2 分钟；单次工具输出和最终结果最多 32 KiB。

取消父任务会取消子请求，但现有文件读取和遍历不保证立即停止。只读工具限制不是文件系统沙箱。当前实现不包含后台子任务、子会话恢复或多 agent 编排。

## MCP

### 配置位置与优先级

Orbit 读取以下两个 JSON 文件：

| 位置 | 范围 | 未写 `enabled` 时的默认值 |
| :--- | :--- | :--- |
| `~/.orbit/.mcp.json` | 用户全局 | 启用 |
| `<当前工作目录>/.mcp.json` | 当前项目 | 禁用 |

同名服务以当前工作目录的配置为准。注意文件名带前导点：`~/.orbit/mcp.json` 和项目 `.orbit/.mcp.json` 不是当前加载位置。

### 本地服务示例

在项目根目录的 `.mcp.json` 中配置：

```json
{
  "mcpServers": {
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/absolute/path/to/allowed/dir"],
      "enabled": true,
      "description": "允许访问的文件目录"
    }
  }
}
```

请替换允许访问的目录，并自行安装 Node.js / `npx`。第三方 MCP 服务的可执行文件和依赖不包含在 Orbit 二进制中。

支持的服务类型：

- `stdio`：使用 `command`、`args`，可设置 `env`；未指定 `type` 时默认使用此类型。
- `http`：Streamable HTTP，使用 `url`，可设置 `headers`。
- `sse`：使用对应 SSE 端点的 `url`，可设置 `headers`。

在 `/mcp` 界面中用方向键或 `j` / `k` 选择服务，按 Enter / Space 切换启用状态，按 Esc / `q` 返回。启停会启动或断开连接、注册或移除工具，并将 `enabled` 写回该服务原本所在的配置文件。

## Skills

Skill 必须定义在 `SKILL.md` 文件中，而不是任意 YAML 或 Markdown 文件。启动时按以下顺序递归发现，同名技能优先使用先发现的定义：

1. `<当前工作目录>/.orbit/skills`
2. `~/.orbit/skills`
3. `~/.agents/skills`
4. `~/.agent/skills`
5. `~/.codex/skills`
6. `~/.claude/skills`

这些用户级兼容目录不表示会扫描项目的 `.claude/skills` 或 `.codex/skills`。新增或修改 Skill 后请重新启动 Orbit，当前未提供实时重载或用户侧的持久化启停开关。

例如创建 `.orbit/skills/repo-map/SKILL.md`：

```markdown
---
name: repo-map
description: 查找项目入口并解释模块关系
---
阅读相关文件，列出 CLI 入口和命令路由，并给出文件路径。
不要修改文件。
```

frontmatter 使用简单的 `name:` 和 `description:` 字段；当前解析器不是完整的 YAML 解析器，不建议使用复杂嵌套格式。

调用示例：

```text
/skills
/skills repo-map 解释当前项目的 CLI 命令路由
```

Skill 调用会向模型提供读取技能文件并执行任务的指令，不是独立的脚本执行器。

## 配置与数据

| 文件或目录 | 用途 |
| :--- | :--- |
| `~/.orbit/Orbitconfig.yaml` | 主配置：语言、主题、推理强度、默认提供商及各提供商配置 |
| `~/.orbit/sessions/<session-id>` | JSON 格式的会话消息历史，文件名不要求 `.json` 后缀 |
| `~/.orbit/memorys.json` | 全局长期记忆 |
| `~/.orbit/dailycost.db` | SQLite 格式的每日 token 与缓存用量统计，不是金额账单 |
| `~/.orbit/.mcp.json` | 用户级 MCP 配置 |
| `<当前工作目录>/.mcp.json` | 项目级 MCP 配置 |
| `<当前工作目录>/.orbit/Orbit.md` | 项目说明与约定，不是模型或鉴权配置文件 |
| `<当前工作目录>/.orbit/skills/` | 项目 Skill 定义 |

可用 `ORBIT_CONFIG_PATH` 指定主 YAML 文件路径，但该变量不会统一迁移会话、记忆、MCP 等其他存储位置。应用配置、MCP 和项目说明有各自的加载规则，不应视为同一套通用的全局 / 项目配置合并机制。

建议通过 `orbit setup` 或 `/provider` 配置提供商。手动配置 API Key 服务时，可参考以下结构，并替换所有占位值：

```yaml
language: zh-CN
think_level: high
default_provider: openai
providers:
  openai:
    auth: apikey
    api_key: YOUR_API_KEY
    base_url: https://api.openai.com/v1
    model: YOUR_MODEL_ID
    type: openai:completions
    default_model: YOUR_MODEL_ID
```

仅在服务端支持 Responses 时改用 `type: openai:responses`。Codex OAuth 请使用登录或导入流程，不要手工复制鉴权信息到公开文件中。API Key、OAuth Token、个人会话和调试记录均不应提交到仓库。

## 项目结构

```text
Orbit-agent/
├── cmd/
│   ├── orbit/          # 主程序
│   └── tui-demo/       # TUI 组件演示
├── internal/
│   ├── agent/          # 主循环、响应解析、上下文压缩和 Subagent
│   ├── agentui/        # Agent 与 TUI 之间的事件接口
│   ├── cli/            # TUI、初始化、命令与配置界面
│   ├── config/         # 配置读取与保存
│   ├── llm/            # 模型请求与协议适配
│   ├── tools/          # 内置工具注册与执行
│   ├── hook/           # 命令确认等执行钩子
│   ├── mcp/            # MCP 连接、启停和工具桥接
│   ├── skills/         # Skill 发现与索引
│   ├── memorys/        # 会话与长期记忆
│   ├── oauth/          # Codex 授权、导入和刷新
│   └── billing/        # Token 使用统计
├── access/             # 静态资源
└── .orbit/             # 本项目的说明等文件
```

上面列出主要模块；其他目录包含提示词、调试记录、主题样式、国际化、事件处理和通用工具。

## 开发与贡献

欢迎提交 [Issue](https://github.com/Motona-Sky/Orbit-agent/issues) 和 [Pull Request](https://github.com/Motona-Sky/Orbit-agent/pulls)。

- 单元测试与被测包放在同一目录，使用 `*_test.go` 后缀。
- 模型和 OAuth 测试应使用假凭据、测试服务器或显式的集成测试；不得读取、刷新或输出开发者的真实 Token。
- 不提交密钥、用户配置、调试记录或构建产物。
- 遵循 Go 格式规范及项目已有代码风格。

```bash
go test ./...
go test -race ./internal/agent ./internal/tools ./internal/llm
go fmt ./...
go vet ./...
go build -o orbit ./cmd/orbit
```

发布打包可运行 `bash ./build.sh`，需要 `tar` 和 `zip`。该脚本会删除并重新创建 `dist`，执行前请确认其中没有需要保留的文件。

## 许可证与交流

本项目采用 [MIT License](LICENSE)。

交流群二维码：

![交流群二维码](access/qrcode_1787224160689.jpg)

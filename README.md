

# 🪐 Orbit Agent

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat-square)](https://github.com)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://github.com)

[English](README_EN.md) | [简体中文](README.md)

[✨ 特性](#-核心特性) • [🚀 快速开始](#-快速开始) • [🛠️ 核心架构](#-架构与机制) • [⌨️ 斜杠命令](#️-交互与斜杠命令) • [🧩 MCP 与技能](#-mcp-与-skills-扩展) • [🤝 贡献](#-贡献指南)

---

## 📖 项目简介

一个简简单单的agent
目前为demo阶段，核心思想为将以循环驱动()

## ✨ 核心特性
- 🧠 **支持codex一键登录**
  - 支持 OpenAI com和resp 兼容协议、Codex OAuth 一键登录与 Token 自动刷新。
  - 支持从codex导入订阅。
- 🛠️ **简约的系统工具与记忆**
  - 简约工具的工具，减少上下文开销。
  - 支持项目与全局记忆分层

- 🔌 **MCP支持**
  - 支持标准远程/本地 MCP 服务端连接与动态工具发现。
  - 自由扩展数据库查询、知识库检索或任意自定义外部服务能力。
- 🧩 **Skills**
  - 自动检测.codex .claude .agent等其他agent的skills，并可以自由开启与关闭
- 💾 **多会话与记忆管理**
  - 基于 SQLite / JSON 的轻量级持久化，支持历史会话自由切换与上下文回滚。
  - 每日 Token 使用与花费统计，账单清晰可查。
- **极致精简**
	- 单二进制实现所有功能


---

## 安装

### 在[官方下载页](https://github.com/Motona-Sky/Orbit-agent/releases)中下载对应的系统版本
| 系统    | 架构    | 安装包  |
| ------- | ------- | ------- |
| Windows | x86/arm | zip包   |
| Linux   | x86/arm | .tar.gz |

安装教程
Windows 

```
运行压缩包内脚本
./install.ps1
```

Linux 

```
解压压缩包
tar -xzf file.tar.gz
chmod +x ./install.sh
./install.sh
```
### 2. 手动安装

#### 克隆仓库
```bash
git clone https://github.com/your-org/Orbit-agent.git
cd Orbit-agent
```
#### 安装依赖
```bash
go install 
```
#### 编译可执行文件
go build -o orbit ./cmd/orbit

#### 添加系统变量

## 快速启动
启动交互式初始化配置，快速设置默认语言、模型提供商与 API Key：

```bash
# 启动全局配置向导
orbit setup

# 单独配置模型与服务商
orbit model
选择codex时，进入codex oauth登录界面
```

### 4. 启动 Agent 对话

```bash
# 直接启动 TUI 交互界面
orbit

# 启用调试模式（输出详细日志）
orbit --debug

# 打开历史会话选择器
orbit -s
# 或
orbit session
```

---

## ⌨️ 交互与斜杠命令

在终端聊天框中输入 `/` 即可触发命令补全弹窗：

| 命令 | 描述 |
| :--- | :--- |
| `/model` | 快速切换或重设当前使用的大语言模型 |
| `/provider` | 配置与选择模型服务商（OpenAI, Codex, 自定义 API 等） |
| `/effort` | 设置思考模型推理强度（`low` / `medium` / `high`） |
| `/skills [name]` | 查看或激活指定工作流技能 (Skills) |
| `/mcp` | 管理与查看已连接的 MCP 服务器和可用工具 |
| `/new` | 归档当前对话并开启全新会话 |
| `/clear` | 清空当前屏显内容（保留上下文） |

---

## 🧩 MCP 与 Skills 扩展

### Model Context Protocol (MCP)
Orbit 原生支持 MCP 协议。你可以在用户配置 `~/.orbit/mcp.json` 或项目级 `.orbit/` 中配置 MCP Servers：

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Skills 技能机制
在 `.orbit/skills/` 目录下放置 YAML 或 Markdown 定义，Orbit 会自动加载并在 `/skills` 中注册，实现常用工程流的指令化驱动。

---

## 🛠️ 架构与机制

```
Orbit-agent/
├── cmd/
│   ├── orbit/              # CLI 主入口
│   └── tui-demo/           # 终端 UI 组件调试预览程序
├── internal/
│   ├── agent/              # Agent 核心运行循环、上下文管理、工具解析
│   ├── agentui/            # 状态与进度 UI 抽象
│   ├── billing/            # 每日消耗与 Token 计费统计
│   ├── cli/                # Bubble Tea TUI 界面与命令路由
│   ├── config/             # 配置管理、持久化与样式定义
│   ├── debug/              # 运行日志记录器
│   ├── event/              # 事件流系统与会话事件
│   ├── i18n/               # 国际化多语言方案 (zh-CN / en)
│   ├── llm/                # LLM 适配器、流式传输、OAuth 认证
│   ├── mcp/                # MCP 客户端协议对接与工具桥接
│   ├── memorys/            # 历史记录、长短时记忆与持久化
│   ├── oauth/              # Codex 登录流程与 Token 刷新机制
│   ├── prompt/             # 预置系统提示词
│   ├── skills/             # 技能动态扫描与索引
│   ├── style/              # 终端样式主题与 ASCII 品牌标识
│   ├── tools/              # 原生工具集 (Exec, Grep, Ls, Read, Update)
│   └── utils/              # 通用工具函数
├── access/                 # 静态资源目录 (Logo, 截图, 演示动图)
└── .orbit/                 # 项目级配置与技能缓存
```

---

## ⚙️ 配置文件说明

Orbit 采用分层配置策略：
- **用户级全局配置**: `~/.orbit/`（存储 API Key、认证 Token、历史会话数据库等敏感数据）
- **项目级配置**: 仓库根目录下的 `.orbit/`（包含针对本仓库的自定义设置与团队共享 Skills）

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！参与贡献前请注意：

1. **测试规范**：单元测试请与对应包放置在同级目录下（`*_test.go`），运行 `go test ./...` 确保全部通过。
2. **安全隔离**：严禁在测试代码、日志输出或提交历史中包含真实 API Token 与鉴权凭证。
3. **代码风格**：遵循 Go 官方编码规范及 Charmbracelet 组件设计哲学。

```bash
# 运行单元测试
go test -v ./...

# 静态格式化检查
go fmt ./...
go vet ./...
```

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 许可协议开源。

<div align="center">
  <sub>Built with ❤️ by Orbit Contributors</sub>
</div>
## 交流群
![qq](access/qrcode_1787224160689.jpg)



## 后续计划
	- 实现未完成功能
	- 补充多agent协作等
	- 实现权限管理
	- 插件系统
	- 循环机制

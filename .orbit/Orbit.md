# Orbit

## 项目简介

Orbit 是一个使用 Go 编写的终端 AI Agent，提供 TUI 交互、模型供应商配置、OAuth、MCP、工具调用、会话记录和多语言支持。

## 技术栈

- Go 1.25
- Bubble Tea / Bubbles / Lip Gloss / Glamour
- OpenAI-compatible HTTP API 与 OAuth
- MCP Go SDK
- YAML 配置
- SQLite

## 开发约定

- 主程序：`cmd/orbit`
- TUI 演示程序：`cmd/tui-demo`
- 应用实现：`internal/`
- 单元测试与被测包放在同一目录，测试文件使用 `_test.go` 后缀。
- 测试不得读取、刷新或输出开发者的真实访问令牌；外部请求应使用测试服务器或显式的集成测试机制。
- 密钥、用户配置、调试日志、构建输出及 Jupyter 检查点不得提交。
- 项目文档可以正常提交，不要在 `.gitignore` 中全局忽略 Markdown 文件。

## 常用命令

```bash
go run ./cmd/orbit
go test ./...
```

## 常见注意事项

- 用户级配置位于 `~/.orbit`，仓库根目录的 `.orbit/` 只用于项目级信息。
- 不要在测试日志中输出 Access Token、Refresh Token 或完整 JWT claims。
- `.ipynb_checkpoints/` 是编辑器生成内容，应始终忽略。

## 最近修改

- 新增英文版说明文档 `README_EN.md`，并在中英文 README 中添加双语互链导航。
- 清理误提交的 Go module 检查点、依赖真实 OAuth 凭据的调试测试和无效占位源码。
- 收紧 `.gitignore`，不再全局忽略 Markdown 或所有以数字 `0` 开头的文件。
- 清理已完成问题遗留的本地调试日志。

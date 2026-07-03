---
name: agent-browser
description: "通过 agent-browser CLI 进行浏览器自动化。当用户需要导航网站、填写表单、点击按钮、截取屏幕截图、提取数据或测试 Web 应用时使用。"
metadata: {"nanobot":{"emoji":"🌐","requires":{"bins":["agent-browser"]},"install":[{"id":"npm","kind":"npm","package":"agent-browser","global":true,"bins":["agent-browser"],"label":"安装 agent-browser (npm)"}]}}
---

# Agent Browser

通过 Chrome/Chromium CDP 进行 CLI 浏览器自动化。安装：`npm i -g agent-browser && agent-browser install`。

**在使用此技能之前**，通过运行 `which agent-browser` 验证工具是否可用。如果找不到命令，请告诉用户浏览器自动化需要 `agent-browser` CLI 和 Chromium，这些仅在重量级容器镜像中可用。不要尝试在运行时安装它。

## 核心工作流

1. `agent-browser open <url>` — 导航
2. `agent-browser snapshot -i` — 获取带引用的交互式元素（`@e1`、`@e2`、...）
3. 使用引用进行交互 — `click @e1`、`fill @e2 "text"`
4. 任何导航或 DOM 更改后重新获取快照 — 引用会失效

```bash
agent-browser open https://example.com/form
agent-browser snapshot -i
# @e1 [input] "Email", @e2 [input] "Password", @e3 [button] "Submit"
agent-browser fill @e1 "user@example.com"
agent-browser fill @e2 "secret"
agent-browser click @e3
agent-browser wait --load networkidle
agent-browser snapshot -i
```

当不需要中间输出时，使用 `&&` 链接命令：
```bash
agent-browser open https://example.com && agent-browser wait --load networkidle && agent-browser snapshot -i
```

## 命令

```bash
# 导航
agent-browser open <url>
agent-browser close

# 快照
agent-browser snapshot -i                # 带引用的交互式元素
agent-browser snapshot -s "#selector"    # 限定到 CSS 选择器

# 交互（使用快照中的 @refs）
agent-browser click @e1
agent-browser fill @e2 "text"            # 清除 + 输入
agent-browser type @e2 "text"            # 不清除直接输入
agent-browser select @e1 "option"
agent-browser check @e1
agent-browser press Enter
agent-browser scroll down 500

# 获取信息
agent-browser get text @e1
agent-browser get url
agent-browser get title

# 等待
agent-browser wait @e1                   # 等待元素
agent-browser wait --load networkidle    # 等待网络空闲
agent-browser wait --url "**/dashboard"  # 等待 URL 模式
agent-browser wait --text "Welcome"      # 等待文本
agent-browser wait 2000                  # 等待毫秒

# 捕获
agent-browser screenshot                 # 截图到临时目录
agent-browser screenshot --full          # 整页
agent-browser screenshot --annotate      # 带编号元素标签（[N] -> @eN）
agent-browser pdf output.pdf

# 语义定位器（当引用不可用时）
agent-browser find text "Sign In" click
agent-browser find label "Email" fill "user@test.com"
agent-browser find role button click --name "Submit"
```

## 认证

```bash
# 选项 1：从用户正在运行的 Chrome 导入
agent-browser --auto-connect state save ./auth.json
agent-browser --state ./auth.json open https://app.example.com

# 选项 2：持久化配置文件
agent-browser --profile ~/.myapp open https://app.example.com/login
# ... 登录一次，以后所有运行都已认证

# 选项 3：会话名称（自动保存/恢复）
agent-browser --session-name myapp open https://app.example.com/login
# ... 登录、关闭，下次运行时会恢复状态

# 选项 4：状态文件
agent-browser state save auth.json
agent-browser state load auth.json
```

## Iframes

Iframe 内容在快照中内联。直接与 iframe 引用交互 —— 无需切换 frame。

## 并行会话

```bash
agent-browser --session s1 open https://site-a.com
agent-browser --session s2 open https://site-b.com
agent-browser session list
```

## JavaScript 执行

```bash
agent-browser eval 'document.title'

# 复杂 JS —— 使用 --stdin 避免 shell 引号问题
agent-browser eval --stdin <<'EVALEOF'
JSON.stringify(Array.from(document.querySelectorAll("a")).map(a => a.href))
EVALEOF
```

## 清理

完成后始终关闭会话：
```bash
agent-browser close
agent-browser --session s1 close
```

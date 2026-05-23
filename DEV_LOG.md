# PicoClaw 二次开发日志 (Dev Log)

> 本仓库基于 [sipeed/picoclaw](https://github.com/sipeed/picoclaw) 进行二次开发。
> 主要分支说明：
>
> - `main`: 定期同步官方 upstream/main，保持纯净。
> - `dev`: 包含所有二次开发功能及合并的官方 PR。

## 🚀 核心差异概览 (Core Differences)

| 模块          | 官方行为                                                        | 二次开发修改                       | 原因/备注                                                                              | 关联分支 / Commit                                                                                                                                                                                 |
| :------------ | :-------------------------------------------------------------- | :--------------------------------- | :------------------------------------------------------------------------------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **UI/会话**   | 仅显示当前渠道                                                  | **实现全渠道会话聚合显示**         | 方便统一管理多平台消息                                                                 | [feat/all-channels-view-only](https://github.com/dtapps/picoclaw/tree/feat/all-channels-view-only)                                                                                                |
| **模型配置**  | 仅支持全局 `max_tokens`                                                    | **支持 per-model `max_tokens`**    | 精细化控制不同模型的输出长度                                                           | [feat/per-model-max-tokens](https://github.com/dtapps/picoclaw/tree/feat/per-model-max-tokens)                                                                                                    |
| **模型逻辑**  | `thinking_level` 获取异常                                       | **修复配置读取逻辑**               | 确保思维链等级正确生效                                                                 | [fix/thinking-level-default-value](https://github.com/dtapps/picoclaw/tree/fix/thinking-level-default-value)                                                                                      |
| **构建/打包** | 标准 Tags                                                       | **默认启用 `whatsapp_native` Tag** | 原生支持 WhatsApp 功能                                                                 | [fix/goreleaser-build](https://github.com/dtapps/picoclaw/tree/fix/goreleaser-build)                                                                                                              |
| **频道**      | 暂无微博频道                                                    | **新增微博频道**                   | 接入微博私信通道                                                                       | [feat/channel-weibo](https://github.com/dtapps/picoclaw/tree/feat/channel-weibo)                                                                                                                  |
| **频道**      | 暂无腾讯元宝频道                                                | **新增腾讯元宝频道**               | 接入腾讯元宝 Bot 通道                                                                  | [feat/channel-yuanbao](https://github.com/dtapps/picoclaw/tree/feat/channel-yuanbao)                                                                                                              |
| **核心逻辑**  | ~~基础来源校验~~                                                | **完善 AllowFrom 字段解析机制**    | 适配新渠道（微博/元宝）时的通用逻辑增强，提升配置兼容性                                | [fix/channel-allowfrom](https://github.com/dtapps/picoclaw/tree/fix/channel-allowfrom)                                                                                                            |
| **定时任务**  | 暂无 UI 管理                                                    | **新增 Web UI 管理页面**           | 可视化创建、编辑、启停定时任务，支持 Cron/Interval/One-time                            | [feat/cron-schedules-ui](https://github.com/dtapps/picoclaw/tree/feat/cron-schedules-ui)                                                                                                          |
| **MCP**       | 暂无 UI 管理                                                    | **新增 Web UI 管理页面**           | 可视化配置 MCP 服务器，支持 stdio/sse/http 类型，含完整参数                            | [feature/mcp-server-config](https://github.com/dtapps/picoclaw/tree/feature/mcp-server-config) - [refactor/mcp-separate-page](https://github.com/dtapps/picoclaw/tree/refactor/mcp-separate-page) |
| **环境变量**  | 暂无 环境变量 管理                                              | **新增环境变量配置 UI**            | 支持全局环境变量管理，自动注入到 Skills 和 MCP 执行环境                                | [feat/env-vars-config](https://github.com/dtapps/picoclaw/tree/feat/env-vars-config)                                                                                                              |
| **工具集**    | [已提交 PR #2691](https://github.com/sipeed/picoclaw/pull/2691) | **新增 `get_current_time` 工具**   | 支持多种格式和时区，方便获取当前时间/日期                                              | [feat/add-get-current-time-tool](https://github.com/dtapps/picoclaw/tree/feat/add-get-current-time-tool)                                                                                          |
| **核心逻辑**  | 空响应无处理                                                    | **新增空响应自动重试机制**         | 当大模型返回空内容或格式异常响应时自动重试，支持配置匹配模式和重试次数，含 Web UI 配置 | [fix/empty-response-retry](https://github.com/dtapps/picoclaw/tree/fix/empty-response-retry)                                                                                                      |
| **模型配置**  | 手动编辑配置文件          | **新增 Web UI 管理页面** | 可视化配置活动模型和备选模型 | [feat/model-settings-ui](https://github.com/dtapps/picoclaw/tree/feat/model-settings-ui) |
| **MCP**       | [已提交 PR #2725](https://github.com/sipeed/picoclaw/pull/2725) | **修复 MCP 初始化失败导致无响应**   | MCP 服务器不可达时降级为警告继续运行，添加 HTTP 30s 超时，避免应用僵尸状态              | [fix/mcp-init-non-fatal](https://github.com/dtapps/picoclaw/tree/fix/mcp-init-non-fatal)                                                                                                          |
| **核心逻辑**  | 非标准 tool_calls 格式无法识别                                  | **新增内联工具调用提取**           | 当大模型（如 kimi-k2）将工具调用写在 content 文本中而非标准 tool_calls 字段时，自动提取并转换，支持配置开关 | [feat/inline-tool-calls](https://github.com/dtapps/picoclaw/tree/feat/inline-tool-calls)                                                                                                          |
| **核心逻辑**  | 模型响应内容含 Anthropic 风格包装和特殊 token                    | **新增模型响应内容清理**           | 当大模型（如 kimi-k2）返回纯文本响应被 `[{'type':'text','text':'...'}]` 包装并附加特殊 token 时，自动提取实际文本内容，支持配置开关 | [feat/clean-anthropic-wrapper](https://github.com/dtapps/picoclaw/tree/feat/clean-anthropic-wrapper) |
| **MCP**       | 仅基础配置，无法查看服务器能力 | **新增服务器详情查看功能** | 可视化查看每个 MCP 服务器的工具列表、提示列表和资源列表 | [feat/mcp-server-details](https://github.com/dtapps/picoclaw/tree/feat/mcp-server-details) |
| **频道**      | 无                                              | **新增 Browser 频道**              | 通过 WebSocket 直连浏览器扩展，支持 Token 认证，无需 Launcher 仪表板                   | [feat/browser-extension](https://github.com/dtapps/picoclaw/tree/feat/browser-extension) |
| **浏览器插件** | 无                                                              | **新增 Chrome 浏览器扩展**         | Sidepanel UI + Background 操作执行 + Content Script 注入，支持点击/输入/导航/截图等操作 | [feat/browser-extension](https://github.com/dtapps/picoclaw/tree/feat/browser-extension) |
| **工具集**    | 无                                      | **新增 `browser_ext` 工具**        | 通过浏览器扩展执行操作（get_page_info/click/type/fill/scroll/screenshot 等），仅限 browser 频道使用 | [feat/browser-extension](https://github.com/dtapps/picoclaw/tree/feat/browser-extension) |
| **工作流**    | 无                                      | **新增工作流引擎**                 | 声明式多步骤任务编排，支持 Cron/事件/手动触发、变量、条件分支、并行执行、重试超时、频道通知，含 Web UI 可视化编辑 | [feature/workflow-engine](https://github.com/dtapps/picoclaw/tree/feature/workflow-engine) |
| **频道**      | [已提交 PR #2893](https://github.com/sipeed/picoclaw/pull/2893) | **新增 SC3Bot 频道**               | 接入 SC3Bot 消息通道，支持接收和发送消息                                                | [feat/sc3bot-channel](https://github.com/dtapps/picoclaw/tree/feat/sc3bot-channel) |
| **模型配置**  | 仅支持全局 `context_window`                                     | **支持 per-model `context_window`** | 每个模型可独立配置上下文窗口大小，未配置时使用 AgentDefaults 的值                     | [feat/per-model-context-window](https://github.com/dtapps/picoclaw/tree/feat/per-model-context-window)                                                                                            |
| **模型配置**  | 仅支持 `context_window` 和 `max_tokens` 控制上下文              | **新增 per-model `max_input_tokens`** | 限制输入 token 数量（包括消息和工具定义），超过则自动截断历史消息，防止超出模型上下文限制 | [feat/per-model-max-input-tokens](https://github.com/dtapps/picoclaw/tree/feat/per-model-max-input-tokens)                                                                                        |

<details>
<summary>📁 已移除的功能 (点击展开)</summary>

| 模块           | 官方行为         | 二次开发修改                   | 原因/备注                        | 关联分支                                                                                       |
| :------------- | :--------------- | :----------------------------- | :------------------------------- | :--------------------------------------------------------------------------------------------- |
| ~~**工具集**~~ | ~~基础搜索工具~~ | ~~**新增"百度百科"搜索工具**~~ | ~~增强中文语境下的知识检索能力~~ | ~~[feat/search-baidu-baike](https://github.com/dtapps/picoclaw/tree/feat/search-baidu-baike)~~ |
| ~~**模型配置**~~ | ~~手动逐个添加模型~~ | ~~**支持快速添加模型预设**~~ | ~~官方已支持 Provider 选择，功能重复~~ | ~~[feat/model-quick-add-presets](https://github.com/dtapps/picoclaw/tree/feat/model-quick-add-presets)~~ |
| ~~**核心逻辑**~~ | ~~LLM 请求无追踪标识~~ | ~~**新增请求追踪（Tracing）**~~ | ~~功能移除~~ | ~~[feature/tracing](https://github.com/dtapps/picoclaw/tree/feature/tracing) ~~ |
| ~~**UI/日志**~~ | ~~日志滚动时出现闪屏~~ | ~~**优化滚动和渲染逻辑**~~ | ~~功能移除~~ | ~~[feature/fix-log-flicker](https://github.com/dtapps/picoclaw/tree/feature/fix-log-flicker) ~~ |

> 百度百科改用 Skill 实现：https://cnb.cool/dtapp/skills

</details>

---

## 🔄 官方 PR 合并记录 (Upstream PR Merges)

记录从官方仓库合并到 `dev` 分支的重要 PR，以便追踪来源。

| 合并日期   | 官方 PR #                                             | 标题/简述                                                             | 合并 Commit Hash                                             | 备注         |
| :--------- | :---------------------------------------------------- | :-------------------------------------------------------------------- | :----------------------------------------------------------- | :----------- |
| 2026-04-11 | [#2460](https://github.com/sipeed/picoclaw/pull/2460) | fix(mcp): send empty object instead of nil arguments in CallTool      | [39e0e59](https://github.com/dtapps/picoclaw/commit/39e0e59) | 已合并未验证 |

---

## 📋 计划合并的 PR (Pending PRs)

记录计划从官方仓库合并但尚未处理的 PR。

| 添加日期   | 官方 PR #                                             | 标题/简述                                    | 优先级 | 备注/计划        |
| :--------- | :---------------------------------------------------- | :------------------------------------------- | :----- | :--------------- |
| 2026-04-29 | [#2413](https://github.com/sipeed/picoclaw/pull/2413) | refactor(line): use official LINE Bot SDK v8 | 低     | 待评估后安排合并 |

---

## 📝 中文支持 (Chinese Localization)

| 模块               | 官方行为          | 二次开发修改                           | 原因/备注                                                                                 | 关联分支                                                                                               |
| :----------------- | :---------------- | :------------------------------------- | :---------------------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------- |
| **Workspace 文件** | 仅英文 `.md`      | **支持中英文双语 `.md`/`.zh.md`**      | 根据 `LANG` 环境变量自动选择语言，中文环境使用 `.zh.md` 文件                              | [feat/translate-workspace-docs](https://github.com/dtapps/picoclaw/tree/feat/translate-workspace-docs) |
| **Skills**         | 仅英文 `SKILL.md` | **所有 Skills 添加中文 `SKILL.zh.md`** | weather, github, tmux, summarize, agent-browser, skill-creator, hardware 等技能完整中文化 | [feat/translate-workspace-docs](https://github.com/dtapps/picoclaw/tree/feat/translate-workspace-docs) |
| **Heartbeat 提示词** | 英文提示词        | **使用 go-i18n 实现国际化**   | `service.go` 中的 `buildPrompt` 和 `createDefaultHeartbeatTemplate` 函数使用 i18n 翻译键，支持中英文自动切换 | [feature/go-i18n-full](https://github.com/dtapps/picoclaw/tree/feature/go-i18n-full) |
| **Agent 系统提示词** | 英文提示词        | **使用 go-i18n 实现国际化**   | `context.go` 中的 `getIdentity`、`BuildSystemPromptParts` 等函数使用 i18n 翻译键（身份介绍、技能目录、多消息输出策略等） | [feature/go-i18n-full](https://github.com/dtapps/picoclaw/tree/feature/go-i18n-full) |
| **Compaction 提示词** | 英文提示词        | **使用 go-i18n 实现国际化**   | `short_compaction.go` 中的 `buildLeafSummaryPrompt`、`buildCondensedSummaryPrompt`、`buildAggressiveLeafSummaryPrompt` 函数使用 i18n 翻译键 | [feature/go-i18n-full](https://github.com/dtapps/picoclaw/tree/feature/go-i18n-full) |
| **Evolution 提示词** | 英文提示词        | **使用 go-i18n 实现国际化**   | `pattern_clusterer.go`、`success_judge.go`、`llm_draft_generator.go`、`skill_draft_policy.go`、`skill_content.go`、`runtime.go`、`drafts.go` 中的 LLM Prompt 和技能生成相关文本使用 i18n 翻译键 | [feature/go-i18n-full](https://github.com/dtapps/picoclaw/tree/feature/go-i18n-full) |
| **i18n 框架** | 无 | **新增 go-i18n 国际化框架** | 使用 `github.com/nicksnyder/go-i18n/v2` 实现，翻译文件使用 TOML 格式，嵌入到二进制中，根据 `LANG` 环境变量自动切换语言 | [feature/go-i18n-full](https://github.com/dtapps/picoclaw/tree/feature/go-i18n-full) |

---

## 📝 功能变更记录 (Change Log)

记录功能的迭代变更历史。

### 2026-05-07 - 模型设置功能优化

**分支**: [feat/model-settings-reorder-fallbacks](https://github.com/dtapps/picoclaw/tree/feat/model-settings-reorder-fallbacks)

基于 [feat/model-settings-ui](https://github.com/dtapps/picoclaw/tree/feat/model-settings-ui) 分支优化：

- **备选模型排序**: 新增上移/下移按钮，支持调整备选模型优先级顺序
- **UI 优化**: 备选模型从标签形式改为列表形式，操作更清晰
- **代码注释**: 为模型设置相关代码添加中文注释

---

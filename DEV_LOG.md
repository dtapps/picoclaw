# PicoClaw 二次开发日志 (Dev Log)

> 本仓库基于 [sipeed/picoclaw](https://github.com/sipeed/picoclaw) 进行二次开发。
> 主要分支说明：
>
> - `main`: 定期同步官方 upstream/main，保持纯净。
> - `dev`: 包含所有二次开发功能及合并的官方 PR。

## 🚀 核心差异概览 (Core Differences)

| 模块          | 官方行为                  | 二次开发修改                       | 原因/备注                    | 关联分支 / Commit                                                                                            |
| :------------ | :------------------------ | :--------------------------------- | :--------------------------- | :----------------------------------------------------------------------------------------------------------- |
| **UI/会话**   | 仅显示当前渠道            | **实现全渠道会话聚合显示**         | 方便统一管理多平台消息       | [feat/all-channels-view-only](https://github.com/dtapps/picoclaw/tree/feat/all-channels-view-only)           |
| **模型配置**  | 全局或固定配置            | **支持 per-model `max_tokens`**    | 精细化控制不同模型的输出长度 | [feat/per-model-max-tokens](https://github.com/dtapps/picoclaw/tree/feat/per-model-max-tokens)               |
| **模型逻辑**  | `thinking_level` 获取异常 | **修复配置读取逻辑**               | 确保思维链等级正确生效       | [fix/thinking-level-default-value](https://github.com/dtapps/picoclaw/tree/fix/thinking-level-default-value) |
| **工具集**    | 基础搜索工具              | **新增“百度百科”搜索工具**         | 增强中文语境下的知识检索能力 | [feat/search-baidu-baike](https://github.com/dtapps/picoclaw/tree/feat/search-baidu-baike)             |
| **构建/打包** | 标准 Tags                 | **默认启用 `whatsapp_native` Tag** | 原生支持 WhatsApp 功能       | [fix/goreleaser-build](https://github.com/dtapps/picoclaw/tree/fix/goreleaser-build)                         |

---

## 🔄 官方 PR 合并记录 (Upstream PR Merges)

记录从官方仓库合并到 `dev` 分支的重要 PR，以便追踪来源。

| 合并日期   | 官方 PR #                                             | 标题/简述                                                             | 合并 Commit Hash                                             | 备注         |
| :--------- | :---------------------------------------------------- | :-------------------------------------------------------------------- | :----------------------------------------------------------- | :----------- |
| 2026-04-11 | [#2460](https://github.com/sipeed/picoclaw/pull/2460) | fix(mcp): send empty object instead of nil arguments in CallTool      | [39e0e59](https://github.com/dtapps/picoclaw/commit/39e0e59) | 已合并未验证 |
| 2026-04-10 | [#2410](https://github.com/sipeed/picoclaw/pull/2410) | feat(tool): add browser automation via Chrome DevTools Protocol (CDP) | [82321c8](https://github.com/dtapps/picoclaw/commit/82321c8) | 已合并未验证 |

---

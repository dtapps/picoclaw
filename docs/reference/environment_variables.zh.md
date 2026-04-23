# 环境变量配置

PicoClaw 支持全局环境变量管理，可以自动注入到 Skills 和 MCP 执行环境中。

## 概述

环境变量配置位于 `config.json` 根级别的 `env_vars` 字段中。这允许你定义以下组件可用的环境变量：

- **Skills**: 通过 exec 工具执行 shell 命令时
- **MCP 服务器**: 启动 MCP 服务器进程时

## 配置结构

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "API_KEY",
        "value": "your-api-key-here",
        "enabled": true,
        "sensitive": true,
        "note": "外部服务的 API 密钥"
      },
      {
        "key": "DEBUG_MODE",
        "value": "true",
        "enabled": true,
        "sensitive": false,
        "note": "启用调试日志"
      }
    ],
    "env_file": "/path/to/.env"
  }
}
```

## 配置字段

### 变量

`variables` 数组中的每个变量支持以下字段：

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `key` | string | 是 | 环境变量名称（必须是有效的环境变量格式） |
| `value` | string | 是 | 环境变量值 |
| `enabled` | bool | 是 | 此变量是否激活 |
| `sensitive` | bool | 否 | 如果为 true，值将在 UI 和日志中隐藏 |
| `note` | string | 否 | 可选的描述或文档 |

### 环境文件

`env_file` 字段指定一个 `.env` 文件的路径，该文件将与配置的变量一起加载。

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `env_file` | string | 否 | 要加载的 `.env` 文件路径 |

## 优先级顺序

解析环境变量时使用以下优先级顺序（从高到低）：

1. **服务器特定的 env**（MCP 服务器配置）- 最高优先级
2. **服务器特定的 env_file**（MCP 服务器配置）
3. **全局 env_vars 变量**（来自此配置）
4. **全局 env_file**（来自此配置）
5. **父进程环境** - 最低优先级

## Web UI 管理

你可以通过 Web UI 管理环境变量：

1. 导航到 **服务 > 环境变量**
2. 添加、编辑、启用/禁用或删除变量
3. 从 `.env` 文件导入
4. 导出到 `.env` 文件
5. 标记为"敏感"的变量将在 UI 中隐藏

## 示例用例

### 外部服务的 API 密钥

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "OPENAI_API_KEY",
        "value": "sk-...",
        "enabled": true,
        "sensitive": true,
        "note": "用于 LLM 调用的 OpenAI API 密钥"
      },
      {
        "key": "ANTHROPIC_API_KEY",
        "value": "sk-ant-...",
        "enabled": true,
        "sensitive": true,
        "note": "Anthropic API 密钥"
      }
    ]
  }
}
```

### 调试和开发设置

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "LOG_LEVEL",
        "value": "debug",
        "enabled": true,
        "sensitive": false,
        "note": "设置日志级别"
      },
      {
        "key": "DISABLE_CACHE",
        "value": "true",
        "enabled": false,
        "sensitive": false,
        "note": "禁用缓存（默认禁用）"
      }
    ]
  }
}
```

### 与 MCP 服务器一起使用

启动 MCP 服务器时自动注入环境变量：

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "GITHUB_TOKEN",
        "value": "ghp_...",
        "enabled": true,
        "sensitive": true
      }
    ]
  },
  "tools": {
    "mcp": {
      "servers": {
        "github": {
          "enabled": true,
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-github"],
          "env": {
            "GITHUB_PERSONAL_ACCESS_TOKEN": "${GITHUB_TOKEN}"
          }
        }
      }
    }
  }
}
```

### 与 Skills 一起使用

使用 exec 工具运行命令时，全局环境变量自动可用：

```bash
# 在 skill 或 exec 命令中
echo $OPENAI_API_KEY  # 将输出配置的值
```

## 安全注意事项

1. **敏感变量**: 将敏感值（API 密钥、令牌、密码）标记为 `"sensitive": true`，以防止它们在 UI 或日志中显示。

2. **文件权限**: 确保你的 `config.json` 文件具有适当的权限（例如 `600`），以防止未经授权的访问。

3. **环境文件**: 如果使用 `env_file`，也要确保它得到适当保护，不要提交到版本控制中。

## 导入和导出

### 从 .env 文件导入

你可以通过 Web UI 导入现有的 `.env` 文件：

1. 转到 **服务 > 环境变量**
2. 点击"导入"
3. 选择你的 `.env` 文件
4. 变量将被解析并添加到配置中

支持的 `.env` 格式：
```bash
# 支持注释
API_KEY=secret_value
DATABASE_URL=postgres://localhost/db

# 带引号的值
DESCRIPTION="This is a description"
```

### 导出到 .env 文件

你可以将配置导出到 `.env` 文件：

1. 转到 **服务 > 环境变量**
2. 点击"导出"
3. 只有启用的变量会被导出
4. 敏感值将以纯文本形式包含在导出的文件中

## 故障排除

### 变量未应用

- 检查变量是否已启用（`"enabled": true`）
- 对于 MCP 服务器：更改变量后重启 MCP 服务器
- 对于 Skills：变量在每次命令执行时应用

### 验证错误

环境变量键必须：
- 以字母或下划线开头
- 仅包含字母、数字和下划线
- 不能为空

### MCP 服务器未接收变量

- MCP 服务器在启动时加载环境变量
- 更改全局 `env_vars` 需要重启 MCP 服务器
- 检查服务器特定的 `env` 和 `env_file` 设置，它们可能会覆盖全局值

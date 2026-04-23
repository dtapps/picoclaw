# Environment Variables Configuration

PicoClaw supports global environment variable management that can be automatically injected into Skills and MCP execution environments.

## Overview

The environment variables configuration is located at the root level of `config.json` under the `env_vars` field. This allows you to define environment variables that will be available to:

- **Skills**: When executing shell commands via the exec tool
- **MCP Servers**: When starting MCP server processes

## Configuration Structure

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "API_KEY",
        "value": "your-api-key-here",
        "enabled": true,
        "sensitive": true,
        "note": "API key for external service"
      },
      {
        "key": "DEBUG_MODE",
        "value": "true",
        "enabled": true,
        "sensitive": false,
        "note": "Enable debug logging"
      }
    ],
    "env_file": "/path/to/.env"
  }
}
```

## Configuration Fields

### Variables

Each variable in the `variables` array supports the following fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | string | Yes | The environment variable name (must be valid env var format) |
| `value` | string | Yes | The environment variable value |
| `enabled` | bool | Yes | Whether this variable is active |
| `sensitive` | bool | No | If true, the value will be masked in UI and logs |
| `note` | string | No | Optional description or documentation |

### Env File

The `env_file` field specifies a path to a `.env` file that will be loaded in addition to the configured variables.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `env_file` | string | No | Path to a `.env` file to load |

## Priority Order

When environment variables are resolved, the following priority order is used (highest to lowest):

1. **Server-specific env** (MCP server config) - Highest priority
2. **Server-specific env_file** (MCP server config)
3. **Global env_vars variables** (from this configuration)
4. **Global env_file** (from this configuration)
5. **Parent process environment** - Lowest priority

## Web UI Management

You can manage environment variables through the Web UI:

1. Navigate to **Services > Environment Variables**
2. Add, edit, enable/disable, or delete variables
3. Import from `.env` files
4. Export to `.env` files
5. Variables marked as "sensitive" will be masked in the UI

## Example Use Cases

### API Keys for External Services

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "OPENAI_API_KEY",
        "value": "sk-...",
        "enabled": true,
        "sensitive": true,
        "note": "OpenAI API key for LLM calls"
      },
      {
        "key": "ANTHROPIC_API_KEY",
        "value": "sk-ant-...",
        "enabled": true,
        "sensitive": true,
        "note": "Anthropic API key"
      }
    ]
  }
}
```

### Debug and Development Settings

```json
{
  "env_vars": {
    "variables": [
      {
        "key": "LOG_LEVEL",
        "value": "debug",
        "enabled": true,
        "sensitive": false,
        "note": "Set logging level"
      },
      {
        "key": "DISABLE_CACHE",
        "value": "true",
        "enabled": false,
        "sensitive": false,
        "note": "Disable caching (disabled by default)"
      }
    ]
  }
}
```

### Using with MCP Servers

Environment variables are automatically injected when MCP servers start:

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

### Using with Skills

When using the exec tool to run commands, global environment variables are automatically available:

```bash
# In a skill or exec command
echo $OPENAI_API_KEY  # Will output the configured value
```

## Security Considerations

1. **Sensitive Variables**: Mark sensitive values (API keys, tokens, passwords) with `"sensitive": true` to prevent them from being displayed in the UI or logs.

2. **File Permissions**: Ensure your `config.json` file has appropriate permissions (e.g., `600`) to prevent unauthorized access.

3. **Environment File**: If using an `env_file`, ensure it is also properly secured and not committed to version control.

## Import and Export

### Import from .env File

You can import existing `.env` files through the Web UI:

1. Go to **Services > Environment Variables**
2. Click "Import"
3. Select your `.env` file
4. Variables will be parsed and added to the configuration

Supported `.env` format:
```bash
# Comments are supported
API_KEY=secret_value
DATABASE_URL=postgres://localhost/db

# Quoted values
DESCRIPTION="This is a description"
```

### Export to .env File

You can export your configuration to a `.env` file:

1. Go to **Services > Environment Variables**
2. Click "Export"
3. Only enabled variables will be exported
4. Sensitive values will be included in plain text in the exported file

## Troubleshooting

### Variables Not Applied

- Check that the variable is enabled (`"enabled": true`)
- For MCP servers: Restart the MCP server after changing variables
- For Skills: Variables are applied on each command execution

### Validation Errors

Environment variable keys must:
- Start with a letter or underscore
- Contain only letters, numbers, and underscores
- Not be empty

### MCP Server Not Receiving Variables

- MCP servers load environment variables at startup time
- Changes to global `env_vars` require restarting the MCP server
- Check server-specific `env` and `env_file` settings which may override global values

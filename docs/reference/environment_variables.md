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
    ]
  }
}
```

## Configuration Fields

### Variables

Each variable in the `variables` array supports the following fields:

| Field       | Type   | Required | Description                                                        |
| ----------- | ------ | -------- | ------------------------------------------------------------------ |
| `key`       | string | Yes      | The environment variable name (must be valid env var format)       |
| `value`     | string | Yes      | The environment variable value                                     |
| `enabled`   | bool   | Yes      | Whether this variable is active                                    |
| `sensitive` | bool   | No       | If true, marks this as sensitive data; the value will be encrypted |
| `note`      | string | No       | Optional description or documentation                              |

## Sensitive Variables

Mark sensitive values (API keys, tokens, passwords) with `"sensitive": true`:

1. **Encrypted Storage**: Sensitive values are encrypted and stored in `.security.yml`

## Priority Order

When environment variables are resolved, the following priority order is used (highest to lowest):

1. **Server-specific env** (MCP server config) - Highest priority
2. **Global env_vars variables** (from this configuration)
3. **Parent process environment** - Lowest priority

## Web UI Management

You can manage environment variables through the Web UI:

1. Navigate to **Services > Environment Variables**
2. Add, edit, enable/disable, or delete variables
3. Variables marked as "sensitive" will be masked as `********` in the UI, click the eye icon to reveal
4. Import from `.env` files
5. Export to `.env` files

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

### Using in Commands

Global environment variables are automatically available when using the exec tool:

```bash
# In a command executed via exec tool
echo $OPENAI_API_KEY  # Will output the configured value
```

## Security Considerations

1. **Sensitive Variables**: Mark sensitive values (API keys, tokens, passwords) with `"sensitive": true` to prevent them from being displayed in the UI or logs.

2. **Encrypted Storage**: Sensitive values are stored in `.security.yml` with AES-GCM encryption.

3. **File Permissions**: Ensure your `config.json` and `.security.yml` files have appropriate permissions (e.g., `600`) to prevent unauthorized access.

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
3. A `.env` file containing all enabled variables will be downloaded

## Technical Details

### Storage

- **Non-sensitive variables**: Stored in plain text in `config.json`
- **Sensitive variables**: Encrypted and stored in `.security.yml`, key derived from system-specific information

### Runtime Injection

Environment variables are automatically injected into all child processes through the isolation runtime. This includes:

- Commands executed via the `exec` tool
- MCP server processes
- Process hooks
- Any other subprocesses spawned by PicoClaw

Changes made through the Web UI take effect immediately without requiring a restart.

package skills

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// InjectEnvVars 将配置的环境变量注入到命令中
func InjectEnvVars(cmd *exec.Cmd, cfg *config.Config) {
	if cfg == nil {
		return
	}

	// 从父环境开始构建环境变量映射
	envMap := make(map[string]string)
	for _, e := range cmd.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			envMap[e[:idx]] = e[idx+1:]
		}
	}

	// 如果指定了环境变量文件，则从文件加载
	if cfg.EnvVars.EnvFile != "" {
		if fileVars, err := config.LoadEnvFile(cfg.EnvVars.EnvFile); err == nil {
			for k, v := range fileVars {
				envMap[k] = v
			}
			logger.DebugF("从环境变量文件加载变量", map[string]any{
				"file":      cfg.EnvVars.EnvFile,
				"var_count": len(fileVars),
			})
		} else {
			logger.WarnF("加载环境变量文件失败", map[string]any{
				"file":  cfg.EnvVars.EnvFile,
				"error": err.Error(),
			})
		}
	}

	// 加载配置的环境变量（优先级高于文件）
	enabledVars := cfg.EnvVars.GetEnabledVars()
	for k, v := range enabledVars {
		envMap[k] = v
	}

	logger.DebugF("Skills 环境变量注入", map[string]any{
		"env_file":    cfg.EnvVars.EnvFile,
		"config_vars": len(enabledVars),
		"total_vars":  len(envMap),
	})

	// 将映射转换为切片
	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
}

// GetEnvVars 返回用于 Skills 执行的环境变量
func GetEnvVars(cfg *config.Config) map[string]string {
	if cfg == nil {
		return nil
	}

	result := make(map[string]string)

	// 从父环境开始
	for _, e := range os.Environ() {
		if idx := strings.Index(e, "="); idx > 0 {
			result[e[:idx]] = e[idx+1:]
		}
	}

	// 如果指定了环境变量文件，则从文件加载
	if cfg.EnvVars.EnvFile != "" {
		if fileVars, err := config.LoadEnvFile(cfg.EnvVars.EnvFile); err == nil {
			for k, v := range fileVars {
				result[k] = v
			}
		}
	}

	// 加载配置的环境变量（更高优先级）
	for k, v := range cfg.EnvVars.GetEnabledVars() {
		result[k] = v
	}

	return result
}

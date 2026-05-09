// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package config

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

const (
	SecurityConfigFile = ".security.yml"
)

// securityPath returns the path to security.yml relative to the config file
func securityPath(configPath string) string {
	configDir := filepath.Dir(configPath)
	return filepath.Join(configDir, SecurityConfigFile)
}

// loadSecurityConfig loads the security configuration from security.yml
// and merges secure field values into the config.
func loadSecurityConfig(cfg *Config, securityPath string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	data, err := os.ReadFile(securityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read security config: %w", err)
	}

	// Save existing channels and ModelList before unmarshal
	savedChannels := make(ChannelsConfig, len(cfg.Channels))
	maps.Copy(savedChannels, cfg.Channels)
	// savedModelList := cfg.ModelList

	// Parse YAML into a yaml.Node tree to extract channels node
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		return fmt.Errorf("failed to parse security config: %w", err)
	}

	// Extract channels node (support both 'channels' and 'channel_list' keys) and env_vars node
	var channelsNode *yaml.Node
	var envVarsNode *yaml.Node
	if len(rootNode.Content) > 0 {
		content := rootNode.Content[0].Content
		for i := 0; i < len(content); i += 2 {
			if i+1 < len(content) {
				key := content[i].Value
				if key == "channels" || key == "channel_list" {
					channelsNode = content[i+1]
				} else if key == "env_vars" {
					envVarsNode = content[i+1]
				}
			}
		}
	}

	// Unmarshal non-channel fields from security.yml
	// This will resolve encrypted values for model_list, tools, etc.
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse security config %s: %w", securityPath, err)
	}
	if err := applyLegacySkillsSecurityConfig(cfg, data); err != nil {
		return fmt.Errorf("failed to parse legacy skills security config: %w", err)
	}

	// Restore channels from saved, then manually merge from security.yml
	cfg.Channels = make(ChannelsConfig)
	maps.Copy(cfg.Channels, savedChannels)

	// If we found a channels node in security.yml, merge it into existing channels
	if channelsNode != nil {
		if err := cfg.Channels.UnmarshalYAML(channelsNode); err != nil {
			return fmt.Errorf("failed to merge channels from security config: %w", err)
		}
	}

	// 如果在 security.yml 中找到 env_vars 节点，将安全值合并到现有环境变量中
	if envVarsNode != nil {
		if err := mergeSecureEnvVars(cfg, envVarsNode); err != nil {
			return fmt.Errorf("从安全配置合并环境变量失败: %w", err)
		}
	}

	return nil
}

func applyLegacySkillsSecurityConfig(cfg *Config, data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root.Content) == 0 {
		return nil
	}

	rootMap := root.Content[0]
	if rootMap == nil || rootMap.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(rootMap.Content); i += 2 {
		keyNode := rootMap.Content[i]
		valueNode := rootMap.Content[i+1]
		if keyNode == nil || valueNode == nil || strings.TrimSpace(keyNode.Value) != "skills" {
			continue
		}
		return applyLegacySkillsSecurityNode(cfg, valueNode)
	}

	return nil
}

func applyLegacySkillsSecurityNode(cfg *Config, skillsNode *yaml.Node) error {
	if cfg == nil || skillsNode == nil || skillsNode.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(skillsNode.Content); i += 2 {
		nameNode := skillsNode.Content[i]
		valueNode := skillsNode.Content[i+1]
		if nameNode == nil || valueNode == nil {
			continue
		}

		name := strings.TrimSpace(nameNode.Value)
		if name == "" || name == "registries" {
			continue
		}

		if name == "github" {
			var legacyGitHub SkillsGithubConfig
			if err := valueNode.Decode(&legacyGitHub); err != nil {
				return err
			}
			if cfg.Tools.Skills.Github.Token.String() == "" && legacyGitHub.Token.String() != "" {
				cfg.Tools.Skills.Github.Token = legacyGitHub.Token
			}
		}

		var legacyRegistry SkillRegistryConfig
		if err := valueNode.Decode(&legacyRegistry); err != nil {
			return err
		}
		legacyRegistry.Name = name
		if legacyRegistry.AuthToken.String() == "" {
			if name == "github" && cfg.Tools.Skills.Github.Token.String() != "" {
				legacyRegistry.AuthToken = cfg.Tools.Skills.Github.Token
			} else {
				continue
			}
		}

		registryCfg, ok := cfg.Tools.Skills.Registries.Get(name)
		if !ok {
			registryCfg = SkillRegistryConfig{
				Name:  name,
				Param: map[string]any{},
			}
		}
		if registryCfg.Param == nil {
			registryCfg.Param = map[string]any{}
		}
		if registryCfg.AuthToken.String() == "" {
			registryCfg.AuthToken = legacyRegistry.AuthToken
		}
		if registryCfg.BaseURL == "" && legacyRegistry.BaseURL != "" {
			registryCfg.BaseURL = legacyRegistry.BaseURL
		}
		for key, value := range legacyRegistry.Param {
			if _, exists := registryCfg.Param[key]; !exists {
				registryCfg.Param[key] = value
			}
		}
		cfg.Tools.Skills.Registries.Set(name, registryCfg)
	}

	return nil
}

// saveSecurityConfig saves the security configuration to security.yml
func saveSecurityConfig(securityPath string, sec *Config) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err := enc.Encode(sec)
	if err != nil {
		return fmt.Errorf("failed to marshal security config: %w", err)
	}
	return fileutil.WriteFileAtomic(securityPath, buf.Bytes(), 0o600)
}

// envVarSecurityEntry 用于保存敏感环境变量到.security.yml
type envVarSecurityEntry struct {
	Key   string       `yaml:"key"`
	Value SecureString `yaml:"value"`
}

// mergeSecureEnvVars 将 security.yml 中的安全环境变量值合并到 cfg 中
func mergeSecureEnvVars(cfg *Config, envVarsNode *yaml.Node) error {
	// security.yml 中的 env_vars 是 envVarSecurityEntry 的序列
	var secureEnvVars []envVarSecurityEntry
	if err := envVarsNode.Decode(&secureEnvVars); err != nil {
		return err
	}

	// 将安全值合并到配置中
	secureValueMap := make(map[string]SecureString)
	for _, entry := range secureEnvVars {
		secureValueMap[entry.Key] = entry.Value
	}

	// 使用安全值更新敏感变量
	for i := range cfg.EnvVars.Variables {
		if cfg.EnvVars.Variables[i].Sensitive {
			if secureValue, ok := secureValueMap[cfg.EnvVars.Variables[i].Key]; ok {
				cfg.EnvVars.Variables[i].SecureValue = secureValue
			}
		}
	}

	return nil
}

// saveEnvVarsToSecurity 仅将敏感环境变量保存到 security.yml
func saveEnvVarsToSecurity(securityPath string, cfg *Config) error {
	// 收集敏感环境变量
	var secureVars []envVarSecurityEntry
	for _, v := range cfg.EnvVars.Variables {
		if v.Sensitive && v.SecureValue.String() != "" {
			secureVars = append(secureVars, envVarSecurityEntry{
				Key:   v.Key,
				Value: v.SecureValue,
			})
		}
	}

	// 如果没有敏感变量，不需要保存
	if len(secureVars) == 0 {
		return nil
	}

	// 读取现有的.security.yml内容
	existingContent, err := os.ReadFile(securityPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取安全配置失败: %w", err)
	}

	// 如果没有现有配置，直接保存环境变量配置
	if len(existingContent) == 0 {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(secureVars); err != nil {
			return fmt.Errorf("序列化环境变量失败: %w", err)
		}
		return fileutil.WriteFileAtomic(securityPath, buf.Bytes(), 0o600)
	}

	// 如果存在现有配置，需要合并
	var existingRoot yaml.Node
	if err := yaml.Unmarshal(existingContent, &existingRoot); err != nil || len(existingRoot.Content) == 0 {
		// 解析失败，直接覆盖
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(secureVars); err != nil {
			return fmt.Errorf("序列化环境变量失败: %w", err)
		}
		return fileutil.WriteFileAtomic(securityPath, buf.Bytes(), 0o600)
	}

	// 解析新的环境变量配置（直接是数组）
	var newBuf bytes.Buffer
	newEnc := yaml.NewEncoder(&newBuf)
	newEnc.SetIndent(2)
	if err := newEnc.Encode(secureVars); err != nil {
		return fmt.Errorf("序列化环境变量失败: %w", err)
	}

	var newRoot yaml.Node
	if err := yaml.Unmarshal(newBuf.Bytes(), &newRoot); err != nil || len(newRoot.Content) == 0 {
		// 解析失败，保留现有配置，忽略错误
		//nolint:nilerr
		return nil
	}

	// 合并：将env_vars节点添加到现有配置或替换
	existingMap := existingRoot.Content[0]

	// 查找并替换或添加env_vars节点
	found := false
	for i := 0; i < len(existingMap.Content); i += 2 {
		if i < len(existingMap.Content) && existingMap.Content[i].Value == "env_vars" {
			if i+1 < len(existingMap.Content) && len(newRoot.Content) > 0 {
				existingMap.Content[i+1] = newRoot.Content[0]
				found = true
				break
			}
		}
	}
	if !found && len(newRoot.Content) > 0 {
		// 添加 env_vars 键和值节点
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "env_vars"}
		existingMap.Content = append(existingMap.Content, keyNode, newRoot.Content[0])
	}

	// 重新编码
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(existingRoot.Content[0]); err != nil {
		return fmt.Errorf("重新编码合并后的配置失败: %w", err)
	}

	return fileutil.WriteFileAtomic(securityPath, buf.Bytes(), 0o600)
}

// SensitiveDataCache caches the strings.Replacer for filtering sensitive data.
// Computed once on first access via sync.Once.
type SensitiveDataCache struct {
	replacer *strings.Replacer
	once     sync.Once
}

// SensitiveDataReplacer returns the strings.Replacer for filtering sensitive data.
// It is computed once on first access via sync.Once.
func (sec *Config) SensitiveDataReplacer() *strings.Replacer {
	sec.initSensitiveCache()
	return sec.sensitiveCache.replacer
}

// initSensitiveCache initializes the sensitive data cache if not already done.
func (sec *Config) initSensitiveCache() {
	if sec.sensitiveCache == nil {
		sec.sensitiveCache = &SensitiveDataCache{}
	}
	sec.sensitiveCache.once.Do(func() {
		values := sec.collectSensitiveValues()
		if len(values) == 0 {
			sec.sensitiveCache.replacer = strings.NewReplacer()
			return
		}

		// Build old/new pairs for strings.Replacer
		var pairs []string
		for _, v := range values {
			if len(v) > 3 {
				pairs = append(pairs, v, "[FILTERED]")
			}
		}
		if len(pairs) == 0 {
			sec.sensitiveCache.replacer = strings.NewReplacer()
			return
		}
		sec.sensitiveCache.replacer = strings.NewReplacer(pairs...)
	})
}

// collectSensitiveValues collects all sensitive strings from SecurityConfig using reflection.
func (sec *Config) collectSensitiveValues() []string {
	var values []string
	collectSensitive(reflect.ValueOf(sec), &values)
	return values
}

// collectSensitive recursively traverses the value and collects SecureString/SecureStrings values.
func collectSensitive(v reflect.Value, values *[]string) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	t := v.Type()

	// Channel: use CollectSensitiveValues() method
	if t == reflect.TypeFor[Channel]() {
		if method := v.MethodByName("CollectSensitiveValues"); method.IsValid() {
			results := method.Call(nil)
			if len(results) > 0 {
				if vals, ok := results[0].Interface().([]string); ok {
					*values = append(*values, vals...)
				}
			}
		}
		return
	}

	// SecureString: collect via String() method (defined on *SecureString)
	if t == reflect.TypeFor[SecureString]() {
		// Create a new pointer to make it addressable for method calls
		ptr := reflect.New(t)
		ptr.Elem().Set(v)
		result := ptr.MethodByName("String").Call(nil)
		if len(result) > 0 {
			if s := result[0].String(); s != "" {
				*values = append(*values, s)
			}
		}
		return
	}

	// SecureStrings ([]*SecureString): iterate and collect each element
	if t == reflect.TypeFor[SecureStrings]() {
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			for elem.Kind() == reflect.Pointer || elem.Kind() == reflect.Interface {
				if elem.IsNil() {
					elem = reflect.Value{}
					break
				}
				elem = elem.Elem()
			}
			if elem.IsValid() && elem.Type() == reflect.TypeFor[SecureString]() {
				result := elem.Addr().MethodByName("String").Call(nil)
				if len(result) > 0 {
					if s := result[0].String(); s != "" {
						*values = append(*values, s)
					}
				}
			}
		}
		return
	}

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !t.Field(i).IsExported() {
				continue
			}
			collectSensitive(v.Field(i), values)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			collectSensitive(v.Index(i), values)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			collectSensitive(v.MapIndex(key), values)
		}
	}
}

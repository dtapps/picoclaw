package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseYAMLWorkflow 将 YAML 字节流反序列化为 Workflow 结构体。
// 用于从 YAML 文件加载工作流定义。
func ParseYAMLWorkflow(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("解析工作流 YAML 失败: %w", err)
	}

	return &wf, nil
}

// renderYAMLWorkflow 将 Workflow 结构体序列化为 YAML 字节流。
// 用于将工作流定义写入 YAML 文件。
func renderYAMLWorkflow(wf *Workflow) ([]byte, error) {
	data, err := yaml.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("生成工作流 YAML 失败: %w", err)
	}
	return data, nil
}

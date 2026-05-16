package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/sipeed/picoclaw/pkg/fileutil"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// PersistStore 管理工作流定义和运行实例状态的磁盘持久化。
// 工作流定义以 YAML 文件存储在 workspace/workflows/ 目录，
// 运行实例状态以 JSON 文件存储在 workspace/state/workflows/ 目录。
// 所有写操作使用 fileutil.WriteFileAtomic 保证原子性。
type PersistStore struct {
	workflowsDir string // 工作流定义目录（workspace/workflows/）
	stateDir     string // 实例状态目录（workspace/state/workflows/）
	mu           sync.RWMutex
}

// legacyStateDir 是旧版状态目录路径，用于数据迁移。
const legacyStateDir = "workflows" + string(filepath.Separator) + ".state"

// NewPersistStore 创建持久化存储实例。
func NewPersistStore(workspaceDir string) *PersistStore {
	return &PersistStore{
		workflowsDir: filepath.Join(workspaceDir, "workflows"),
		stateDir:     filepath.Join(workspaceDir, "state", "workflows"),
	}
}

// Init 创建目录结构（如果不存在），并迁移旧版状态数据。
func (ps *PersistStore) Init() error {
	for _, dir := range []string{ps.workflowsDir, ps.stateDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	// 迁移旧版 workflows/.state → stateworkflows
	return ps.migrateLegacyState()
}

// migrateLegacyState 将旧版 workflows/.state 目录中的文件迁移到 stateworkflows。
// 迁移完成后删除空目录，失败不影响启动。
func (ps *PersistStore) migrateLegacyState() error {
	// 旧版状态目录在 workflows/.state，用 workflowsDir 推导
	legacyDir := filepath.Join(filepath.Dir(ps.workflowsDir), legacyStateDir)
	if _, err := os.Stat(legacyDir); os.IsNotExist(err) {
		return nil // 旧目录不存在，无需迁移
	}

	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		// 无法读取则跳过，不阻塞启动
		return err
	}
	if len(entries) == 0 {
		os.Remove(legacyDir) // 空目录直接删除
		return nil
	}

	moved := 0
	for _, entry := range entries {
		src := filepath.Join(legacyDir, entry.Name())
		dst := filepath.Join(ps.stateDir, entry.Name())

		// 目标已存在则跳过（避免覆盖新数据）
		if _, err2 := os.Stat(dst); err2 == nil {
			continue
		}

		if err2 := os.Rename(src, dst); err2 != nil {
			continue // 单个文件失败继续处理下一个
		}
		moved++
	}

	if moved > 0 {
		logger.InfoCF("workflow", "工作流状态目录迁移完成", map[string]any{
			"from":     legacyDir,
			"to":       ps.stateDir,
			"migrated": moved,
		})
	}

	_ = os.Remove(legacyDir) // 尝试清理空目录
	return nil
}

// LoadAllWorkflows 从 workflows 目录读取所有 YAML 工作流定义。
// 返回以名称为键的工作流映射。格式错误的文件会被跳过。
func (ps *PersistStore) LoadAllWorkflows() (map[string]*Workflow, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	entries, err := os.ReadDir(ps.workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Workflow), nil
		}
		return nil, err
	}

	workflows := make(map[string]*Workflow)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 只处理 .yml 和 .yaml 文件
		if filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(ps.workflowsDir, name))
		if err != nil {
			continue
		}

		wf, err := ParseYAMLWorkflow(data)
		if err != nil {
			continue
		}

		// 根据 .disabled 标记文件判断是否启用
		wf.Enabled = !ps.isDisabled(wf.Name)
		workflows[wf.Name] = wf
	}

	return workflows, nil
}

// SaveWorkflow 将工作流定义写入 YAML 文件。
func (ps *PersistStore) SaveWorkflow(wf *Workflow) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// 保存前自动迁移到最新版本
	if wf.MigrateToV2() {
		logger.DebugCF("workflow", "保存前自动迁移工作流配置",
			map[string]any{"workflow": wf.Name, "version": wf.Version})
	}

	data, err := renderYAMLWorkflow(wf)
	if err != nil {
		return err
	}

	filename := sanitizeName(wf.Name) + ".yml"
	return fileutil.WriteFileAtomic(filepath.Join(ps.workflowsDir, filename), data, 0o644)
}

// GetWorkflowsDirModTime 返回工作流目录的最新修改时间。
// 用于检查工作流文件是否有变化，避免不必要的重新加载。
func (ps *PersistStore) GetWorkflowsDirModTime() (int64, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	info, err := os.Stat(ps.workflowsDir)
	if err != nil {
		return 0, err
	}
	return info.ModTime().Unix(), nil
}

// LoadSingleWorkflow 从磁盘读取指定名称的工作流定义。
// 返回最新磁盘数据，用于避免用过期内存状态覆写外部修改。
func (ps *PersistStore) LoadSingleWorkflow(name string) (*Workflow, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	filename := sanitizeName(name) + ".yml"
	data, err := os.ReadFile(filepath.Join(ps.workflowsDir, filename))
	if err != nil {
		return nil, fmt.Errorf("读取工作流 '%s' 失败: %w", name, err)
	}

	wf, err := ParseYAMLWorkflow(data)
	if err != nil {
		return nil, fmt.Errorf("解析工作流 '%s' 失败: %w", name, err)
	}

	wf.Enabled = !ps.isDisabled(wf.Name)
	return wf, nil
}

// WorkflowExists 检查指定名称的工作流是否已存在。
func (ps *PersistStore) WorkflowExists(name string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ymlPath := filepath.Join(ps.workflowsDir, sanitizeName(name)+".yml")
	_, err := os.Stat(ymlPath)
	return err == nil
}

// DeleteWorkflow 删除工作流定义文件及其状态。
func (ps *PersistStore) DeleteWorkflow(name string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// 删除 YAML 定义文件
	filename := sanitizeName(name) + ".yml"
	ymlPath := filepath.Join(ps.workflowsDir, filename)
	if err := os.Remove(ymlPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 删除禁用标记文件
	disabledPath := filepath.Join(ps.workflowsDir, sanitizeName(name)+".disabled")
	_ = os.Remove(disabledPath)

	// 删除运行状态
	statePath := filepath.Join(ps.stateDir, sanitizeName(name)+".json")
	_ = os.Remove(statePath)

	return nil
}

// SetEnabled 持久化工作流的启用/禁用状态。
// 通过创建/删除 .disabled 标记文件实现。
func (ps *PersistStore) SetEnabled(name string, enabled bool) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	disabledPath := filepath.Join(ps.workflowsDir, sanitizeName(name)+".disabled")
	if enabled {
		if err := os.Remove(disabledPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(disabledPath, []byte{}, 0o644)
}

// isDisabled 检查工作流是否有 .disabled 标记文件。
func (ps *PersistStore) isDisabled(name string) bool {
	disabledPath := filepath.Join(ps.workflowsDir, sanitizeName(name)+".disabled")
	_, err := os.Stat(disabledPath)
	return err == nil
}

// --- 实例状态持久化 ---

// SaveInstance 将工作流运行实例持久化到 state 目录。
// 使用原子写入保证数据一致性。
func (ps *PersistStore) SaveInstance(inst *WorkflowInstance) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	data, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}

	// 文件名格式：{工作流名}_{实例ID}.json
	filename := sanitizeName(inst.WorkflowName) + "_" + inst.ID + ".json"
	return fileutil.WriteFileAtomic(filepath.Join(ps.stateDir, filename), data, 0o600)
}

// LoadInstances 读取指定工作流的所有运行实例。
func (ps *PersistStore) LoadInstances(workflowName string) ([]*WorkflowInstance, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	entries, err := os.ReadDir(ps.stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	prefix := sanitizeName(workflowName) + "_"
	var instances []*WorkflowInstance
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		// 只加载匹配工作流前缀的实例文件
		if len(entry.Name()) < len(prefix) || entry.Name()[:len(prefix)] != prefix {
			continue
		}

		data, err := os.ReadFile(filepath.Join(ps.stateDir, entry.Name()))
		if err != nil {
			continue
		}

		var inst WorkflowInstance
		if err := json.Unmarshal(data, &inst); err != nil {
			continue
		}
		instances = append(instances, &inst)
	}

	// 按 started_at 倒序排列（最新的在前）
	slices.SortFunc(instances, func(a, b *WorkflowInstance) int {
		if a.StartedAt.Before(b.StartedAt) {
			return 1
		} else if a.StartedAt.After(b.StartedAt) {
			return -1
		}
		return 0
	})

	return instances, nil
}

// LoadInstance 读取单个运行实例。
func (ps *PersistStore) LoadInstance(workflowName, instanceID string) (*WorkflowInstance, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	filename := sanitizeName(workflowName) + "_" + instanceID + ".json"
	data, err := os.ReadFile(filepath.Join(ps.stateDir, filename))
	if err != nil {
		return nil, err
	}

	var inst WorkflowInstance
	if err := json.Unmarshal(data, &inst); err != nil {
		return nil, err
	}
	return &inst, nil
}

// PurgeOldInstances 清理超出保留数量的旧实例文件。
func (ps *PersistStore) PurgeOldInstances(workflowName string, keepCount int) error {
	instances, err := ps.LoadInstances(workflowName)
	if err != nil || len(instances) <= keepCount {
		return err
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// LoadInstances 返回倒序（最新在前），因此尾部是旧实例
	// 删除最早的实例，保留最近的 keepCount 个
	for i := keepCount; i < len(instances); i++ {
		filename := sanitizeName(workflowName) + "_" + instances[i].ID + ".json"
		_ = os.Remove(filepath.Join(ps.stateDir, filename))
	}
	return nil
}

// DeleteInstance 删除指定工作流的单个运行实例。
func (ps *PersistStore) DeleteInstance(workflowName, instanceID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	filename := sanitizeName(workflowName) + "_" + instanceID + ".json"
	return os.Remove(filepath.Join(ps.stateDir, filename))
}

// sanitizeName 将工作流名称转换为安全的文件名前缀。
// 仅保留合法字符，空格转连字符，移除特殊字符。
// 注意：不进行大小写转换，以避免 MyWorkflow 和 myworkflow 在不区分大小写的文件系统上碰撞。
func sanitizeName(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' {
			result = append(result, '-')
		}
	}
	if len(result) == 0 {
		return "unnamed"
	}
	return string(result)
}

package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/audio/asr"
	"github.com/sipeed/picoclaw/pkg/audio/tts"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	_ "github.com/sipeed/picoclaw/pkg/channels/browser"
	_ "github.com/sipeed/picoclaw/pkg/channels/deltachat"
	_ "github.com/sipeed/picoclaw/pkg/channels/dingtalk"
	_ "github.com/sipeed/picoclaw/pkg/channels/discord"
	_ "github.com/sipeed/picoclaw/pkg/channels/feishu"
	_ "github.com/sipeed/picoclaw/pkg/channels/irc"
	_ "github.com/sipeed/picoclaw/pkg/channels/line"
	_ "github.com/sipeed/picoclaw/pkg/channels/maixcam"
	_ "github.com/sipeed/picoclaw/pkg/channels/mqtt"
	_ "github.com/sipeed/picoclaw/pkg/channels/onebot"
	_ "github.com/sipeed/picoclaw/pkg/channels/pico"
	_ "github.com/sipeed/picoclaw/pkg/channels/qq"
	_ "github.com/sipeed/picoclaw/pkg/channels/sc3bot"
	_ "github.com/sipeed/picoclaw/pkg/channels/slack"
	_ "github.com/sipeed/picoclaw/pkg/channels/slack_webhook"
	_ "github.com/sipeed/picoclaw/pkg/channels/teams_webhook"
	_ "github.com/sipeed/picoclaw/pkg/channels/telegram"
	_ "github.com/sipeed/picoclaw/pkg/channels/vk"
	_ "github.com/sipeed/picoclaw/pkg/channels/wecom"
	_ "github.com/sipeed/picoclaw/pkg/channels/weibo"
	_ "github.com/sipeed/picoclaw/pkg/channels/weixin"
	_ "github.com/sipeed/picoclaw/pkg/channels/whatsapp"
	_ "github.com/sipeed/picoclaw/pkg/channels/whatsapp_native"
	_ "github.com/sipeed/picoclaw/pkg/channels/yuanbao"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
	"github.com/sipeed/picoclaw/pkg/devices"
	runtimeevents "github.com/sipeed/picoclaw/pkg/events"
	"github.com/sipeed/picoclaw/pkg/health"
	"github.com/sipeed/picoclaw/pkg/heartbeat"
	"github.com/sipeed/picoclaw/pkg/i18n"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/media"
	"github.com/sipeed/picoclaw/pkg/netbind"
	"github.com/sipeed/picoclaw/pkg/pid"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/state"
	"github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
	"github.com/sipeed/picoclaw/pkg/utils"
	"github.com/sipeed/picoclaw/pkg/workflow"
)

const (
	serviceShutdownTimeout  = 30 * time.Second
	providerReloadTimeout   = 30 * time.Second
	gracefulShutdownTimeout = 15 * time.Second

	logPath   = "logs"
	panicFile = "gateway_panic.log"
	logFile   = "gateway.log"
)

type services struct {
	CronService      *cron.CronService
	WorkflowService  *workflow.Service
	HeartbeatService *heartbeat.HeartbeatService
	MediaStore       media.MediaStore
	ChannelManager   *channels.Manager
	DeviceService    *devices.Service
	HealthServer     *health.Server
	VoiceAgentCancel context.CancelFunc
	manualReloadChan chan struct{}
	reloading        atomic.Bool
	authToken        string
}

type startupBlockedProvider struct {
	reason string
}

func logChannelVoiceCapabilities(cm *channels.Manager, asrAvailable bool, ttsAvailable bool) {
	if cm == nil {
		return
	}

	names := cm.GetEnabledChannels()
	sort.Strings(names)
	for _, name := range names {
		ch, ok := cm.GetChannel(name)
		if !ok {
			continue
		}
		caps := channels.DetectVoiceCapabilities(name, ch, asrAvailable, ttsAvailable)
		logger.InfoCF("voice", i18n.T("channel_voice_capabilities"), map[string]any{
			"channel": name,
			"asr":     caps.ASR,
			"tts":     caps.TTS,
		})
	}
}

func (p *startupBlockedProvider) Chat(
	_ context.Context,
	_ []providers.Message,
	_ []providers.ToolDefinition,
	_ string,
	_ map[string]any,
) (*providers.LLMResponse, error) {
	return nil, fmt.Errorf("%s", p.reason)
}

func (p *startupBlockedProvider) GetDefaultModel() string {
	return ""
}

// Run starts the gateway runtime using the configuration loaded from configPath.
func Run(debug bool, homePath, configPath string, allowEmptyStartup bool) (runErr error) {
	startedAt := time.Now()
	panicPath := filepath.Join(homePath, logPath, panicFile)
	panicFunc, err := logger.InitPanic(panicPath)
	if err != nil {
		return fmt.Errorf("error initializing panic log: %w", err)
	}
	defer panicFunc()

	if err = logger.EnableFileLogging(filepath.Join(homePath, logPath, logFile)); err != nil {
		logger.Fatal(fmt.Sprintf("error enabling file logging: %v", err))
	}
	defer logger.DisableFileLogging()

	if debug {
		logger.SetLevel(logger.DEBUG)
	} else {
		logger.SetLevelFromString(config.ResolveGatewayLogLevel(configPath))
	}
	defer func() {
		if runErr != nil {
			logger.ErrorCF("gateway", "Gateway startup failed", map[string]any{
				"config_path": configPath,
				"error":       runErr.Error(),
				"home_path":   homePath,
				"allow_empty": allowEmptyStartup,
				"debug":       debug,
			})
		}
	}()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if err = preCheckConfig(cfg); err != nil {
		return fmt.Errorf("config pre-check failed: %w", err)
	}

	// Debug mode permanently overrides the config log level to DEBUG.
	if debug {
		fmt.Println("🔍 Debug mode enabled")
	} else {
		effectiveLogLevel := config.EffectiveGatewayLogLevel(cfg)
		logger.SetLevelFromString(effectiveLogLevel)
		logger.Infof("Log level set to %q", effectiveLogLevel)
	}

	bindPlan, listenResult, err := openGatewayListeners(cfg.Gateway.Host, cfg.Gateway.Port)
	if err != nil {
		return fmt.Errorf("error opening gateway listeners: %w", err)
	}

	// Enforce singleton: write PID file with generated token.
	pidData, err := pid.WritePidFile(homePath, bindPlan.ProbeHost, cfg.Gateway.Port)
	if err != nil {
		logger.Warnf("write pid file failed: %v", err)
		for _, ln := range listenResult.Listeners {
			_ = ln.Close()
		}
		return fmt.Errorf("singleton check failed: %w", err)
	}
	defer pid.RemovePidFile(homePath)
	closeListeners := true
	defer func() {
		if !closeListeners {
			return
		}
		for _, ln := range listenResult.Listeners {
			_ = ln.Close()
		}
	}()

	provider, modelID, err := createStartupProvider(cfg, allowEmptyStartup)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)
	msgBus.SetEventPublisher(agentLoop.RuntimeEventBus())
	publishGatewayEvent(agentLoop, runtimeevents.KindGatewayStart, startedAt, nil)

	fmt.Println("\n📦 Agent Status:")
	startupStatus := collectGatewayStartupStatus(agentLoop.GetStartupInfo())
	fmt.Printf("  • Tools: %d loaded\n", startupStatus.toolsCount)
	fmt.Printf("  • Skills: %d/%d available\n", startupStatus.skillsAvailable, startupStatus.skillsTotal)

	logger.InfoCF("agent", "Agent initialized", startupStatus.logFields)

	runningServices, err := setupAndStartServices(cfg, agentLoop, msgBus, pidData.Token, listenResult)
	if err != nil {
		return err
	}
	// All services (channels + shared HTTP server) are up; mark the health
	// server ready so GET /ready reports "ready". The health endpoints are
	// mounted on the shared gateway mux, so Health.Server.Start() (which would
	// otherwise set this) is never called — we flip the flag explicitly here.
	runningServices.HealthServer.SetReady(true)
	publishGatewayEvent(agentLoop, runtimeevents.KindGatewayReady, startedAt, nil)
	closeListeners = false

	// Setup manual reload channel for /reload endpoint
	manualReloadChan := make(chan struct{}, 1)
	runningServices.manualReloadChan = manualReloadChan
	reloadTrigger := func() error {
		if !runningServices.reloading.CompareAndSwap(false, true) {
			return fmt.Errorf("reload already in progress")
		}
		select {
		case manualReloadChan <- struct{}{}:
			return nil
		default:
			// Should not happen, but reset flag if channel is full
			runningServices.reloading.Store(false)
			return fmt.Errorf("reload already queued")
		}
	}
	runningServices.HealthServer.SetReloadFunc(reloadTrigger)
	agentLoop.SetReloadFunc(reloadTrigger)

	for _, bindHost := range listenResult.BindHosts {
		fmt.Printf("✓ Gateway started on %s\n", net.JoinHostPort(bindHost, strconv.Itoa(cfg.Gateway.Port)))
	}
	fmt.Println("Press Ctrl+C to stop")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go agentLoop.Run(ctx)

	var configReloadChan <-chan *config.Config
	stopWatch := func() {}
	if cfg.Gateway.HotReload {
		configReloadChan, stopWatch = setupConfigWatcherPolling(configPath, debug)
		logger.Info("Config hot reload enabled")
	}
	defer stopWatch()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			logger.Info("Shutting down...")
			shutdownGateway(runningServices, agentLoop, provider, msgBus, true)
			return nil
		case newCfg := <-configReloadChan:
			if !runningServices.reloading.CompareAndSwap(false, true) {
				logger.Warn("Config reload skipped: another reload is in progress")
				continue
			}
			err := executeReload(ctx, agentLoop, newCfg, &provider, runningServices, msgBus, allowEmptyStartup, debug)
			if err != nil {
				logger.Errorf("Config reload failed: %v", err)
			}
		case <-manualReloadChan:
			logger.Info("Manual reload triggered via /reload endpoint")
			newCfg, err := config.LoadConfig(configPath)
			if err != nil {
				logger.Errorf("Error loading config for manual reload: %v", err)
				runningServices.reloading.Store(false)
				continue
			}
			if err = newCfg.ValidateModelList(); err != nil {
				logger.Errorf("Config validation failed: %v", err)
				runningServices.reloading.Store(false)
				continue
			}
			err = executeReload(ctx, agentLoop, newCfg, &provider, runningServices, msgBus, allowEmptyStartup, debug)
			if err != nil {
				logger.Errorf("Manual reload failed: %v", err)
			} else {
				logger.Info("Manual reload completed successfully")
			}
		}
	}
}

func preCheckConfig(cfg *config.Config) error {
	if cfg.Gateway.Port <= 0 || cfg.Gateway.Port > 65535 {
		return fmt.Errorf("invalid gateway port: %d, port must be between 1 and 65535", cfg.Gateway.Port)
	}
	return nil
}

type gatewayStartupStatus struct {
	toolsCount      int
	skillsAvailable int
	skillsTotal     int
	logFields       map[string]any
}

func collectGatewayStartupStatus(startupInfo map[string]any) gatewayStartupStatus {
	status := gatewayStartupStatus{logFields: map[string]any{}}

	if toolsInfo, ok := startupInfo["tools"].(map[string]any); ok {
		if count, ok := startupInfoInt(toolsInfo["count"]); ok {
			status.toolsCount = count
			status.logFields["tools_count"] = count
		}
	}

	if skillsInfo, ok := startupInfo["skills"].(map[string]any); ok {
		if total, ok := startupInfoInt(skillsInfo["total"]); ok {
			status.skillsTotal = total
			status.logFields["skills_total"] = total
		}
		if available, ok := startupInfoInt(skillsInfo["available"]); ok {
			status.skillsAvailable = available
			status.logFields["skills_available"] = available
		}
	}

	return status
}

func startupInfoInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func executeReload(
	ctx context.Context,
	agentLoop *agent.AgentLoop,
	newCfg *config.Config,
	provider *providers.LLMProvider,
	runningServices *services,
	msgBus *bus.MessageBus,
	allowEmptyStartup bool,
	debug bool,
) (err error) {
	startedAt := time.Now()
	publishGatewayEvent(agentLoop, runtimeevents.KindGatewayReloadStarted, startedAt, nil)
	defer runningServices.reloading.Store(false)
	defer func() {
		if err != nil {
			publishGatewayEvent(agentLoop, runtimeevents.KindGatewayReloadFailed, startedAt, err)
			return
		}
		publishGatewayEvent(agentLoop, runtimeevents.KindGatewayReloadCompleted, startedAt, nil)
	}()

	err = handleConfigReload(ctx, agentLoop, newCfg, provider, runningServices, msgBus, allowEmptyStartup, debug)
	return err
}

func createStartupProvider(
	cfg *config.Config,
	allowEmptyStartup bool,
) (providers.LLMProvider, string, error) {
	modelName := cfg.Agents.Defaults.GetModelName()
	if modelName == "" && allowEmptyStartup {
		reason := "no default model configured; gateway started in limited mode"
		fmt.Printf("⚠ Warning: %s\n", reason)
		logger.WarnCF("gateway", "Gateway started without default model", map[string]any{
			"limited_mode": true,
		})
		return &startupBlockedProvider{reason: reason}, "", nil
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return nil, "", err
	}
	return provider, modelID, nil
}

func setupAndStartServices(
	cfg *config.Config,
	agentLoop *agent.AgentLoop,
	msgBus *bus.MessageBus,
	authToken string,
	listenResult netbind.OpenResult,
) (*services, error) {
	runningServices := &services{}

	execTimeout := time.Duration(cfg.Tools.Cron.ExecTimeoutMinutes) * time.Minute
	var err error
	runningServices.CronService, err = setupCronTool(
		agentLoop,
		msgBus,
		cfg.WorkspacePath(),
		cfg.Agents.Defaults.RestrictToWorkspace,
		execTimeout,
		cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up cron service: %w", err)
	}
	if err = runningServices.CronService.Start(); err != nil {
		return nil, fmt.Errorf("error starting cron service: %w", err)
	}
	fmt.Println("✓ Cron service started")

	runningServices.HeartbeatService = heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		cfg.Heartbeat.Interval,
		cfg.Heartbeat.Enabled,
	)
	runningServices.HeartbeatService.SetBus(msgBus)
	runningServices.HeartbeatService.SetHandler(createHeartbeatHandler(agentLoop))
	if err = runningServices.HeartbeatService.Start(); err != nil {
		return nil, fmt.Errorf("error starting heartbeat service: %w", err)
	}
	fmt.Println("✓ Heartbeat service started")

	runningServices.MediaStore = media.NewFileMediaStoreWithCleanup(media.MediaCleanerConfig{
		Enabled:  cfg.Tools.MediaCleanup.Enabled,
		MaxAge:   time.Duration(cfg.Tools.MediaCleanup.MaxAge) * time.Minute,
		Interval: time.Duration(cfg.Tools.MediaCleanup.Interval) * time.Minute,
	})
	if fms, ok := runningServices.MediaStore.(*media.FileMediaStore); ok {
		fms.Start()
	}

	runningServices.ChannelManager, err = channels.NewManager(
		cfg,
		msgBus,
		runningServices.MediaStore,
		channels.WithRuntimeEvents(agentLoop.RuntimeEventBus()),
	)
	if err != nil {
		if fms, ok := runningServices.MediaStore.(*media.FileMediaStore); ok {
			fms.Stop()
		}
		return nil, fmt.Errorf("error creating channel manager: %w", err)
	}

	// 工作流服务（在 ChannelManager 之后创建，以便使用同步消息发送）
	runningServices.WorkflowService, err = setupWorkflowService(
		agentLoop,
		msgBus,
		runningServices.ChannelManager,
		cfg.WorkspacePath(),
		cfg,
	)
	if err != nil {
		return nil, fmt.Errorf("error setting up workflow service: %w", err)
	}
	agentLoop.SetWorkflowService(runningServices.WorkflowService)
	fmt.Println("✓ Workflow service started")

	agentLoop.SetChannelManager(runningServices.ChannelManager)
	agentLoop.SetMediaStore(runningServices.MediaStore)

	transcriber := asr.DetectTranscriber(cfg)
	if transcriber != nil {
		agentLoop.SetTranscriber(transcriber)
		logger.InfoCF("voice", "Transcription enabled (agent-level)", map[string]any{"provider": transcriber.Name()})
	}

	ttsAvailable := tts.DetectTTS(cfg) != nil

	enabledChannels := runningServices.ChannelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("✓ Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("⚠ Warning: No channels enabled")
	}

	runningServices.authToken = authToken
	runningServices.HealthServer = health.NewServer(listenResult.ProbeHost, cfg.Gateway.Port, authToken)

	var listenAddr string
	if len(listenResult.Listeners) > 0 {
		listenAddr = listenResult.Listeners[0].Addr().String()
	} else {
		listenAddr = net.JoinHostPort(listenResult.ProbeHost, strconv.Itoa(cfg.Gateway.Port))
	}
	runningServices.ChannelManager.SetupHTTPServerListeners(
		listenResult.Listeners,
		listenAddr,
		runningServices.HealthServer,
	)

	// 注册工作流内部 API 端点（供 Web 后端反向代理调用）
	if runningServices.WorkflowService != nil {
		workflowAPI := workflow.NewInternalAPI(runningServices.WorkflowService)
		workflowAPI.SetToolLister(&agentToolLister{agentLoop: agentLoop})
		workflowAPI.RegisterOnMux(runningServices.ChannelManager.Mux())
	}

	if err = runningServices.ChannelManager.StartAll(context.Background()); err != nil {
		return nil, fmt.Errorf("error starting channels: %w", err)
	}

	logChannelVoiceCapabilities(runningServices.ChannelManager, transcriber != nil, ttsAvailable)

	if transcriber != nil {
		// Start Voice Agent Orchestrator after channels are ready.
		vaCtx, vaCancel := context.WithCancel(context.Background())
		runningServices.VoiceAgentCancel = vaCancel
		voiceAgent := asr.NewAgent(msgBus, transcriber)
		voiceAgent.Start(vaCtx)
	}

	healthAddr := net.JoinHostPort(listenResult.ProbeHost, strconv.Itoa(cfg.Gateway.Port))
	fmt.Printf(
		"✓ Health endpoints available at http://%s/health, /ready and /reload (POST)\n",
		healthAddr,
	)

	stateManager := state.NewManager(cfg.WorkspacePath())
	runningServices.DeviceService = devices.NewService(devices.Config{
		Enabled:    cfg.Devices.Enabled,
		MonitorUSB: cfg.Devices.MonitorUSB,
	}, stateManager)
	runningServices.DeviceService.SetBus(msgBus)
	if err = runningServices.DeviceService.Start(context.Background()); err != nil {
		logger.ErrorCF("device", "Error starting device service", map[string]any{"error": err.Error()})
	} else if cfg.Devices.Enabled {
		fmt.Println("✓ Device event service started")
	}

	return runningServices, nil
}

func stopAndCleanupServices(runningServices *services, shutdownTimeout time.Duration, isReload bool) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// reload should not stop channel manager
	if !isReload && runningServices.ChannelManager != nil {
		runningServices.ChannelManager.StopAll(shutdownCtx)
	}
	if runningServices.VoiceAgentCancel != nil {
		runningServices.VoiceAgentCancel()
	}
	if runningServices.DeviceService != nil {
		runningServices.DeviceService.Stop()
	}
	if runningServices.HeartbeatService != nil {
		runningServices.HeartbeatService.Stop()
	}
	if runningServices.CronService != nil {
		runningServices.CronService.Stop()
	}
	if runningServices.MediaStore != nil {
		if fms, ok := runningServices.MediaStore.(*media.FileMediaStore); ok {
			fms.Stop()
		}
	}
}

func shutdownGateway(
	runningServices *services,
	agentLoop *agent.AgentLoop,
	provider providers.LLMProvider,
	msgBus *bus.MessageBus,
	fullShutdown bool,
) {
	publishGatewayEvent(agentLoop, runtimeevents.KindGatewayShutdown, time.Time{}, nil)

	if cp, ok := provider.(providers.StatefulProvider); ok && fullShutdown {
		cp.Close()
	}

	stopAndCleanupServices(runningServices, gracefulShutdownTimeout, false)

	if fullShutdown && msgBus != nil {
		msgBus.Close()
	}

	agentLoop.Stop()
	agentLoop.Close()

	logger.Info("✓ Gateway stopped")
}

func handleConfigReload(
	ctx context.Context,
	al *agent.AgentLoop,
	newCfg *config.Config,
	providerRef *providers.LLMProvider,
	runningServices *services,
	msgBus *bus.MessageBus,
	allowEmptyStartup bool,
	debug bool,
) error {
	logger.Info("🔄 Config file changed, reloading...")

	newModel := newCfg.Agents.Defaults.ModelName

	logger.Infof(" New model is '%s', recreating provider...", newModel)

	logger.Info("  Stopping all services...")
	stopAndCleanupServices(runningServices, serviceShutdownTimeout, true)

	newProvider, newModelID, err := createStartupProvider(newCfg, allowEmptyStartup)
	if err != nil {
		logger.Errorf("  ⚠ Error creating new provider: %v", err)
		logger.Warn("  Attempting to restart services with old provider and config...")
		if restartErr := restartServices(al, runningServices, msgBus); restartErr != nil {
			logger.Errorf("  ⚠ Failed to restart services: %v", restartErr)
		}
		return fmt.Errorf("error creating new provider: %w", err)
	}

	if newModelID != "" {
		newCfg.Agents.Defaults.ModelName = newModelID
	}

	reloadCtx, reloadCancel := context.WithTimeout(context.Background(), providerReloadTimeout)
	defer reloadCancel()

	if err := al.ReloadProviderAndConfig(reloadCtx, newProvider, newCfg); err != nil {
		logger.Errorf("  ⚠ Error reloading agent loop: %v", err)
		if cp, ok := newProvider.(providers.StatefulProvider); ok {
			cp.Close()
		}
		logger.Warn("  Attempting to restart services with old provider and config...")
		if restartErr := restartServices(al, runningServices, msgBus); restartErr != nil {
			logger.Errorf("  ⚠ Failed to restart services: %v", restartErr)
		}
		return fmt.Errorf("error reloading agent loop: %w", err)
	}

	*providerRef = newProvider

	logger.Info("  Restarting all services with new configuration...")
	if err := restartServices(al, runningServices, msgBus); err != nil {
		logger.Errorf("  ⚠ Error restarting services: %v", err)
		return fmt.Errorf("error restarting services: %w", err)
	}

	logger.Info("  ✓ Provider, configuration, and services reloaded successfully (thread-safe)")

	// Debug mode permanently overrides the config log level to DEBUG.
	if !debug {
		// Update log level last so that reload-related info/warn logs above are not suppressed.
		effectiveLogLevel := config.EffectiveGatewayLogLevel(newCfg)
		logger.SetLevelFromString(effectiveLogLevel)
		logger.Infof("Log level changing from current to %q", effectiveLogLevel)
	}

	return nil
}

func restartServices(
	al *agent.AgentLoop,
	runningServices *services,
	msgBus *bus.MessageBus,
) error {
	cfg := al.GetConfig()

	execTimeout := time.Duration(cfg.Tools.Cron.ExecTimeoutMinutes) * time.Minute
	var err error
	runningServices.CronService, err = setupCronTool(
		al,
		msgBus,
		cfg.WorkspacePath(),
		cfg.Agents.Defaults.RestrictToWorkspace,
		execTimeout,
		cfg,
	)
	if err != nil {
		return fmt.Errorf("error restarting cron service: %w", err)
	}
	if err = runningServices.CronService.Start(); err != nil {
		return fmt.Errorf("error restarting cron service: %w", err)
	}
	fmt.Println("  ✓ Cron service restarted")

	// 工作流服务（传入 ChannelManager 以便使用同步消息发送）
	runningServices.WorkflowService, err = setupWorkflowService(
		al,
		msgBus,
		runningServices.ChannelManager,
		cfg.WorkspacePath(),
		cfg,
	)
	if err != nil {
		return fmt.Errorf("error restarting workflow service: %w", err)
	}
	al.SetWorkflowService(runningServices.WorkflowService)

	// 重新注册工作流内部 API（更新服务引用）
	if runningServices.ChannelManager.Mux() != nil {
		workflowAPI := workflow.NewInternalAPI(runningServices.WorkflowService)
		workflowAPI.SetToolLister(&agentToolLister{agentLoop: al})
		workflowAPI.RegisterOnMux(runningServices.ChannelManager.Mux())
	}

	fmt.Println("  ✓ Workflow service restarted")

	runningServices.HeartbeatService = heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		cfg.Heartbeat.Interval,
		cfg.Heartbeat.Enabled,
	)
	runningServices.HeartbeatService.SetBus(msgBus)
	runningServices.HeartbeatService.SetHandler(createHeartbeatHandler(al))
	if err = runningServices.HeartbeatService.Start(); err != nil {
		return fmt.Errorf("error restarting heartbeat service: %w", err)
	}
	fmt.Println("  ✓ Heartbeat service restarted")

	runningServices.MediaStore = media.NewFileMediaStoreWithCleanup(media.MediaCleanerConfig{
		Enabled:  cfg.Tools.MediaCleanup.Enabled,
		MaxAge:   time.Duration(cfg.Tools.MediaCleanup.MaxAge) * time.Minute,
		Interval: time.Duration(cfg.Tools.MediaCleanup.Interval) * time.Minute,
	})
	if fms, ok := runningServices.MediaStore.(*media.FileMediaStore); ok {
		fms.Start()
	}
	if runningServices.ChannelManager != nil {
		runningServices.ChannelManager.SetMediaStore(runningServices.MediaStore)
	}
	al.SetMediaStore(runningServices.MediaStore)

	al.SetChannelManager(runningServices.ChannelManager)

	if err = runningServices.ChannelManager.Reload(context.Background(), cfg); err != nil {
		return fmt.Errorf("error reload channels: %w", err)
	}
	fmt.Println("  ✓ Channels restarted.")

	enabledChannels := runningServices.ChannelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("  ✓ Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("  ⚠ Warning: No channels enabled")
	}

	stateManager := state.NewManager(cfg.WorkspacePath())
	runningServices.DeviceService = devices.NewService(devices.Config{
		Enabled:    cfg.Devices.Enabled,
		MonitorUSB: cfg.Devices.MonitorUSB,
	}, stateManager)
	runningServices.DeviceService.SetBus(msgBus)
	if err := runningServices.DeviceService.Start(context.Background()); err != nil {
		logger.WarnCF("device", "Failed to restart device service", map[string]any{"error": err.Error()})
	} else if cfg.Devices.Enabled {
		fmt.Println("  ✓ Device event service restarted")
	}

	transcriber := asr.DetectTranscriber(cfg)
	al.SetTranscriber(transcriber)
	if transcriber != nil {
		logger.InfoCF("voice", "Transcription re-enabled (agent-level)", map[string]any{"provider": transcriber.Name()})

		// Start Voice Agent Orchestrator on reload
		vaCtx, vaCancel := context.WithCancel(context.Background())
		runningServices.VoiceAgentCancel = vaCancel
		voiceAgent := asr.NewAgent(msgBus, transcriber)
		voiceAgent.Start(vaCtx)
	} else {
		logger.InfoCF("voice", "Transcription disabled", nil)
	}

	ttsAvailable := tts.DetectTTS(cfg) != nil
	logChannelVoiceCapabilities(runningServices.ChannelManager, transcriber != nil, ttsAvailable)
	// NOTE: PID file is written once at startup and not updated on reload.
	// Changing the gateway listen address requires a full restart.

	return nil
}

func setupConfigWatcherPolling(configPath string, debug bool) (chan *config.Config, func()) {
	configChan := make(chan *config.Config, 1)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		lastModTime := getFileModTime(configPath)
		lastSize := getFileSize(configPath)

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentModTime := getFileModTime(configPath)
				currentSize := getFileSize(configPath)

				if currentModTime.After(lastModTime) || currentSize != lastSize {
					if debug {
						logger.Debugf("🔍 Config file change detected")
					}

					time.Sleep(500 * time.Millisecond)

					lastModTime = currentModTime
					lastSize = currentSize

					newCfg, err := config.LoadConfig(configPath)
					if err != nil {
						logger.Errorf("⚠ Error loading new config: %v", err)
						logger.Warn("  Using previous valid config")
						continue
					}

					if err := newCfg.ValidateModelList(); err != nil {
						logger.Errorf("  ⚠ New config validation failed: %v", err)
						logger.Warn("  Using previous valid config")
						continue
					}

					logger.Info("✓ Config file validated and loaded")

					select {
					case configChan <- newCfg:
					default:
						logger.Warn("⚠ Previous config reload still in progress, skipping")
					}
				}
			case <-stop:
				return
			}
		}
	}()

	stopFunc := func() {
		close(stop)
		wg.Wait()
	}

	return configChan, stopFunc
}

func getFileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func setupCronTool(
	agentLoop *agent.AgentLoop,
	msgBus *bus.MessageBus,
	workspace string,
	restrict bool,
	execTimeout time.Duration,
	cfg *config.Config,
) (*cron.CronService, error) {
	cronStorePath := filepath.Join(workspace, "cron", "jobs.json")

	cronService := cron.NewCronService(cronStorePath, nil)

	var cronTool *tools.CronTool
	if cfg.Tools.IsToolEnabled("cron") {
		var err error
		cronTool, err = tools.NewCronTool(cronService, agentLoop, msgBus, workspace, restrict, execTimeout, cfg)
		if err != nil {
			return nil, fmt.Errorf("critical error during CronTool initialization: %w", err)
		}

		agentLoop.RegisterTool(cronTool)
	}

	if cronTool != nil {
		cronService.SetOnJob(func(job *cron.CronJob) (string, error) {
			result := cronTool.ExecuteJob(context.Background(), job)
			return result, nil
		})
	}

	return cronService, nil
}

// setupWorkflowService 初始化并启动工作流服务。
// 创建持久化存储、步骤执行器、引擎和服务实例，
// 注册 WorkflowTool 到 Agent（如果启用），并启动服务。
// 设计模式与 setupCronTool 保持一致。
func setupWorkflowService(
	agentLoop *agent.AgentLoop,
	msgBus *bus.MessageBus,
	channelManager *channels.Manager,
	workspace string,
	cfg *config.Config,
) (*workflow.Service, error) {
	// 创建持久化存储
	store := workflow.NewPersistStore(workspace)

	// 创建步骤执行器，注入 Agent 提示和工具调用的回调函数
	executor := &workflow.StepExecutor{
		// agent_prompt 类型步骤的回调：通过 AgentLoop 直接执行提示词
		AgentPromptFunc: func(ctx context.Context, prompt string) (string, error) {
			// 从上下文中提取 session key，如果没有则创建新的
			sessionKey := fmt.Sprintf("agent:workflow-%s", uuid.New().String())
			if sk, ok := workflow.SessionKeyFromCtx(ctx); ok && sk != "" {
				sessionKey = sk
				logger.InfoCF("workflow", "AgentPromptFunc 使用复用的 SessionKey", map[string]any{
					"session_key": sessionKey,
				})
			} else {
				logger.InfoCF("workflow", "AgentPromptFunc 生成新 SessionKey（未从 context 获取到）", map[string]any{
					"session_key": sessionKey,
				})
			}
			// 从上下文中提取频道信息，由 Engine 在执行时注入
			channel := "cli"
			chatID := "workflow"
			if ch, ok := workflow.ChannelFromCtx(ctx); ok {
				channel = ch
			}
			if cid, ok := workflow.ChatIDFromCtx(ctx); ok {
				chatID = cid
			}

			// 根据 Step.Tools 配置决定是否发送工具列表
			// tools: off 时禁用工具，其他值或默认时发送工具
			if noTools, has := workflow.NoToolsFromCtx(ctx); has && noTools {
				ctx = agent.WithNoTools(ctx, true)
			}

			// 根据 Step.Skills 配置决定是否加载技能
			// skills: off 时禁用技能，其他值或默认时加载技能
			if noSkills, has := workflow.NoSkillsFromCtx(ctx); has && noSkills {
				ctx = agent.WithSuppressSkills(ctx, true)
			}

			// 根据 Workflow 配置的 history 和 system_prompt 设置上下文
			if history, ok := workflow.HistoryFromCtx(ctx); ok && history == "off" {
				ctx = agent.WithNoHistory(ctx, true)
			}
			if systemPrompt, ok := workflow.SystemPromptFromCtx(ctx); ok && systemPrompt == "off" {
				ctx = agent.WithSuppressSystemPrompt(ctx, true)
			}

			return agentLoop.ProcessDirectWithChannel(ctx, prompt, sessionKey, channel, chatID)
		},
		// tool_call 类型步骤的回调：通过工具注册表查找并执行指定工具
		ToolCallFunc: func(ctx context.Context, toolName string, args map[string]any) (string, bool, error) {
			registry := agentLoop.GetRegistry()
			defaultAgent := registry.GetDefaultAgent()
			if defaultAgent == nil {
				return "", false, fmt.Errorf("no default agent available")
			}

			var tool tools.Tool
			var ok bool
			if tool, ok = defaultAgent.Tools.Get(toolName); !ok {
				for _, agentID := range registry.ListAgentIDs() {
					if agentID == defaultAgent.ID {
						continue
					}
					if agent, found := registry.GetAgent(agentID); found {
						if t, found := agent.Tools.Get(toolName); found {
							tool = t
							ok = true
							break
						}
					}
				}
			}
			if !ok {
				return "", false, fmt.Errorf("tool '%s' not found", toolName)
			}
			ctx = toolshared.WithToolContext(ctx, "cli", "workflow")
			result := tool.Execute(ctx, args)
			output := result.ForLLM
			// 如果 ForLLM 是占位符（大文本被省略），尝试从 ArtifactTags 读取真实数据
			if strings.Contains(output, "omitted from model context") && len(result.ArtifactTags) > 0 {
				// 从 ArtifactTags 中提取文件路径并读取内容
				for _, tag := range result.ArtifactTags {
					if strings.HasPrefix(tag, "[file:") && strings.HasSuffix(tag, "]") {
						path := strings.TrimPrefix(tag, "[file:")
						path = strings.TrimSuffix(path, "]")
						if data, err := os.ReadFile(path); err == nil {
							output = string(data)
							break
						}
					}
				}
			}
			return output, result.IsError, nil
		},
	}

	// 创建引擎
	engine := workflow.NewEngine(store, executor)
	engine.SetEventBus(agentLoop.RuntimeEventBus())

	// sendNotification 通过 msgBus.PublishOutbound 发送工作流通知消息，
	// 消息进入消息总线 → dispatchOutbound → worker queue，保证 FIFO 顺序。
	// 标记 message_kind=workflow_notification 使 preSend 跳过 streamActive/placeholder 检查。
	sendNotification := func(channelName, chatID, content string) {
		logger.DebugCF("workflow", "准备发送通知到渠道", map[string]any{
			"channel":     channelName,
			"chat_id":     chatID,
			"content_len": len(content),
		})
		msg := bus.OutboundMessage{
			Context: bus.NewOutboundContext(channelName, chatID, ""),
			Content: content,
		}
		if msg.Context.Raw == nil {
			msg.Context.Raw = make(map[string]string, 1)
		}
		msg.Context.Raw["message_kind"] = "workflow_notification"
		err := msgBus.PublishOutbound(context.Background(), msg)
		if err != nil {
			logger.ErrorCF("workflow", "发送通知失败", map[string]any{
				"channel": channelName,
				"chat_id": chatID,
				"error":   err.Error(),
			})
		} else {
			logger.InfoCF("workflow", "通知已发送到消息总线", map[string]any{
				"channel": channelName,
				"chat_id": chatID,
			})
		}
	}

	// sendToAllTargets 发送通知到所有目标频道
	sendToAllTargets := func(targets []workflow.NotifyTarget, content string) {
		logger.DebugCF("workflow", "开始发送多频道通知", map[string]any{
			"target_count": len(targets),
			"targets":      targets,
		})
		for i, target := range targets {
			if target.Channel != "" && target.ChatID != "" {
				logger.InfoCF("workflow", fmt.Sprintf("发送通知到频道 [%d/%d]", i+1, len(targets)), map[string]any{
					"channel": target.Channel,
					"chat_id": target.ChatID,
				})
				sendNotification(target.Channel, target.ChatID, content)
				// 等待一小段时间，让异步发送有机会完成
				time.Sleep(100 * time.Millisecond)
			} else {
				logger.WarnCF("workflow", "跳过无效的通知目标", map[string]any{
					"index":   i,
					"channel": target.Channel,
					"chat_id": target.ChatID,
				})
			}
		}
	}

	// sendToInstTargets 根据实例配置发送通知（支持多频道）
	sendToInstTargets := func(inst *workflow.WorkflowInstance, content string) {
		logger.DebugCF("workflow", "准备发送工作流通知", map[string]any{
			"workflow":        inst.WorkflowName,
			"instance_id":     inst.ID,
			"notify_channels": inst.NotifyChannels,
		})
		if len(inst.NotifyChannels) > 0 {
			logger.DebugCF("workflow", "使用 NotifyChannels 多频道模式", map[string]any{
				"count": len(inst.NotifyChannels),
			})
			sendToAllTargets(inst.NotifyChannels, content)
		} else {
			logger.WarnCF("workflow", "没有配置任何通知目标，跳过通知", map[string]any{
				"workflow": inst.WorkflowName,
			})
		}
	}

	// 设置执行开始回调：推送开始通知到绑定频道
	engine.SetOnStart(func(inst *workflow.WorkflowInstance) <-chan struct{} {
		sendToInstTargets(inst,
			fmt.Sprintf("🚀 工作流 '%s' 开始执行\n触发: %s", inst.WorkflowName, inst.TriggerType))
		return nil
	})

	// 设置步骤开始回调：通知频道即将执行的步骤
	engine.SetOnStepStart(
		func(step workflow.Step, inst *workflow.WorkflowInstance, resolvedPrompt string, resolvedArgs map[string]any) {
			// 检查是否启用开始通知（nil 或 true 为启用）
			if step.NotifyOnStart != nil && !*step.NotifyOnStart {
				return
			}

			var content string
			switch step.Action {
			case "agent_prompt":
				promptToShow := resolvedPrompt
				if promptToShow == "" {
					promptToShow = step.Prompt
				}
				actionDesc := fmt.Sprintf("Agent 提示: %s", promptToShow)
				if len(actionDesc) > 80 {
					actionDesc = actionDesc[:77] + "..."
				}
				content = fmt.Sprintf("▶️ Agent 步骤 '%s' 开始执行（%s）", workflow.StepLabel(step), actionDesc)
			case "parallel":
				content = fmt.Sprintf("▶️ 并行步骤 '%s' 开始执行（并行执行）", workflow.StepLabel(step))
			case "if":
				actionDesc := fmt.Sprintf("条件判断: %s", step.When)
				if len(actionDesc) > 80 {
					actionDesc = actionDesc[:77] + "..."
				}
				content = fmt.Sprintf("▶️ 条件步骤 '%s' 开始执行（%s）", workflow.StepLabel(step), actionDesc)
			case "tool_call":
				toolFeedbackMaxLen := cfg.Agents.Defaults.GetToolFeedbackMaxArgsLength()
				var argsPreview string
				if resolvedArgs != nil {
					argsPreview = utils.FormatArgsJSON(resolvedArgs, true, false)
				} else {
					argsPreview = utils.FormatArgsJSON(step.Args, true, false)
				}
				argsPreview = utils.Truncate(argsPreview, toolFeedbackMaxLen)
				stepLabel := workflow.StepLabel(step)
				content = utils.FormatToolFeedbackMessage(
					step.Tool,
					fmt.Sprintf("▶️ 工具步骤 '%s' 开始执行", stepLabel),
					argsPreview,
				)
			default:
				content = fmt.Sprintf("▶️ 步骤 '%s' 开始执行（%s）", workflow.StepLabel(step), step.Action)
			}

			sendToInstTargets(inst, content)
		},
	)

	// 设置步骤完成回调：将结果实时推送到绑定频道
	engine.SetOnStepComplete(func(step workflow.Step, inst *workflow.WorkflowInstance, result workflow.StepResult) {
		// notify 步骤：直接发送消息（不显示步骤完成标记），不受完成通知开关控制
		if step.Action == "notify" {
			if result.Error != nil {
				return
			}
			message := result.Output
			if message == "" {
				message = step.Message
			}
			if message != "" {
				sendToInstTargets(inst, message)
			}
			return
		}

		// agent_prompt 步骤：始终推送 AI 响应（主要输出）
		if step.Action == "agent_prompt" {
			if result.Error == nil && result.Output != "" {
				sendToInstTargets(inst, result.Output)
			}
			// 继续检查是否需要发送"步骤完成"额外通知
		}

		// 检查完成通知开关（nil 或 true 为启用）
		if step.NotifyOnComplete != nil && !*step.NotifyOnComplete {
			return
		}

		// agent_prompt 步骤已处理过 AI 响应，这里只发送"步骤完成"额外通知
		if step.Action == "agent_prompt" {
			stepLabel := workflow.StepLabel(step)
			if result.Error != nil {
				sendToInstTargets(inst, fmt.Sprintf("❌ Agent 步骤 '%s' 执行失败", stepLabel))
			} else {
				sendToInstTargets(inst, fmt.Sprintf("✅ Agent 步骤 '%s' 执行完成", stepLabel))
			}
			return
		}

		if step.Action == "tool_call" {
			stepLabel := workflow.StepLabel(step)
			var resultMsg string
			if result.Error != nil {
				statusText := fmt.Sprintf("❌ 工具步骤 '%s' 执行失败", stepLabel)
				resultMsg = utils.FormatToolFeedbackMessage(step.Tool, statusText, result.Error.Error())
			} else if result.Output != "" {
				statusText := fmt.Sprintf("✅ 工具步骤 '%s' 执行完成", stepLabel)
				outputPreview := result.Output
				if len([]rune(outputPreview)) > 300 {
					outputPreview = string([]rune(outputPreview)[:297]) + "..."
				}
				resultMsg = utils.FormatToolFeedbackMessage(step.Tool, statusText, outputPreview)
			} else {
				statusText := fmt.Sprintf("✅ 工具步骤 '%s' 执行完成", stepLabel)
				resultMsg = utils.FormatToolFeedbackMessage(step.Tool, statusText, "")
			}
			sendToInstTargets(inst, resultMsg)
			return
		}

		// parallel / if 步骤的完成通知
		stepLabel := workflow.StepLabel(step)
		if step.Action == "parallel" {
			if result.Error != nil {
				sendToInstTargets(inst, fmt.Sprintf("❌ 并行步骤 '%s' 执行失败", stepLabel))
			} else {
				sendToInstTargets(inst, fmt.Sprintf("✅ 并行步骤 '%s' 执行完成", stepLabel))
			}
		} else if step.Action == "if" {
			if result.Error != nil {
				sendToInstTargets(inst, fmt.Sprintf("❌ 条件步骤 '%s' 执行失败", stepLabel))
			} else {
				sendToInstTargets(inst, fmt.Sprintf("✅ 条件步骤 '%s' 执行完成", stepLabel))
			}
		} else {
			if result.Error != nil {
				sendToInstTargets(inst, fmt.Sprintf("❌ 步骤 '%s' 执行失败", stepLabel))
			} else {
				sendToInstTargets(inst, fmt.Sprintf("✅ 步骤 '%s' 执行完成", stepLabel))
			}
		}
	})

	// 设置执行完成回调：将结果推送到绑定频道
	engine.SetOnComplete(func(inst *workflow.WorkflowInstance) <-chan struct{} {
		statusText := "✅ 完成"
		if inst.Status == workflow.StatusFailed {
			statusText = "❌ 失败"
		} else if inst.Status == workflow.StatusCancelled {
			statusText = "⛔ 已取消"
		}
		summary := fmt.Sprintf("工作流 '%s' %s\n实例: %s\n耗时: %s",
			inst.WorkflowName, statusText, inst.ID,
			inst.FinishedAt.Sub(inst.StartedAt).Round(time.Second))
		if inst.Error != "" {
			summary += "\n错误: " + inst.Error
		}
		sendToInstTargets(inst, summary)
		return nil
	})

	// 创建服务
	svcCfg := workflow.ServiceConfig{
		WorkspaceDir: workspace,
		MsgBus:       msgBus,
		EventBus:     agentLoop.RuntimeEventBus(),
		ToolSchema: func(toolName string) (map[string]any, bool) {
			registry := agentLoop.GetRegistry()
			defaultAgent := registry.GetDefaultAgent()
			if defaultAgent == nil {
				return nil, false
			}
			tool, ok := defaultAgent.Tools.Get(toolName)
			if !ok {
				return nil, false
			}
			return tool.Parameters(), true
		},
	}
	service := workflow.NewService(store, engine, svcCfg)

	// 如果工作流工具已启用，注册到 Agent
	if cfg.Tools.IsToolEnabled("workflow") {
		workflowTool, err := tools.NewWorkflowTool(service)
		if err != nil {
			return nil, fmt.Errorf("critical error during WorkflowTool initialization: %w", err)
		}
		agentLoop.RegisterTool(workflowTool)
	}

	// 启动服务（加载工作流定义、订阅事件总线、启动 cron 检查循环）
	if err := service.Start(); err != nil {
		return nil, fmt.Errorf("error starting workflow service: %w", err)
	}

	return service, nil
}

// agentToolLister 实现 workflow.ToolLister 接口，从 agent registry 获取已注册工具名称。
type agentToolLister struct {
	agentLoop *agent.AgentLoop
}

func (l *agentToolLister) ListRegisteredTools() []string {
	if l.agentLoop == nil {
		return nil
	}
	registry := l.agentLoop.GetRegistry()
	seen := make(map[string]struct{})
	var tools []string
	for _, agentID := range registry.ListAgentIDs() {
		if ag, ok := registry.GetAgent(agentID); ok {
			for _, name := range ag.Tools.List() {
				if _, exists := seen[name]; !exists {
					seen[name] = struct{}{}
					tools = append(tools, name)
				}
			}
		}
	}
	return tools
}

func createHeartbeatHandler(agentLoop *agent.AgentLoop) func(prompt, channel, chatID string) *tools.ToolResult {
	return func(prompt, channel, chatID string) *tools.ToolResult {
		if channel == "" || chatID == "" {
			channel, chatID = "cli", "direct"
		}

		response, err := agentLoop.ProcessHeartbeat(context.Background(), prompt, channel, chatID)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Heartbeat error: %v", err))
		}
		if response == "HEARTBEAT_OK" {
			return tools.SilentResult("Heartbeat OK")
		}
		return tools.SilentResult(response)
	}
}

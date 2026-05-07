// PicoClaw 浏览器扩展 - 设置页面脚本
// 处理设置管理和配置

// DOM 元素
const serverUrlInput = document.getElementById('serverUrl');
const apiKeyInput = document.getElementById('apiKey');
const autoConnectCheckbox = document.getElementById('autoConnect');
const showNotificationsCheckbox = document.getElementById('showNotifications');
const confirmActionsCheckbox = document.getElementById('confirmActions');
const highlightElementsCheckbox = document.getElementById('highlightElements');
const maxHistoryInput = document.getElementById('maxHistory');
const sendPageInfoCheckbox = document.getElementById('sendPageInfo');
const excludePrivateCheckbox = document.getElementById('excludePrivate');
const testConnectionBtn = document.getElementById('testConnection');
const saveConnectionBtn = document.getElementById('saveConnection');
const clearHistoryBtn = document.getElementById('clearHistory');
const resetSettingsBtn = document.getElementById('resetSettings');
const saveAllBtn = document.getElementById('saveAll');
const connectionResult = document.getElementById('connectionResult');
const viewLogsLink = document.getElementById('viewLogs');
const logsModal = document.getElementById('logsModal');
const closeLogsBtn = document.getElementById('closeLogs');
const clearLogsBtn = document.getElementById('clearLogs');
const downloadLogsBtn = document.getElementById('downloadLogs');
const logsContent = document.getElementById('logsContent');

// 默认设置
// PicoClaw Gateway 默认端口为 18790
const defaultSettings = {
  serverUrl: 'http://localhost:18790',
  apiKey: '',
  autoConnect: false,
  showNotifications: true,
  confirmActions: false,
  highlightElements: true,
  maxHistory: 50,
  sendPageInfo: true,
  excludePrivate: true
};

// 初始化
async function init() {
  await loadSettings();
  setupEventListeners();
}

// 设置事件监听器
function setupEventListeners() {
  // 连接设置
  testConnectionBtn.addEventListener('click', testConnection);
  saveConnectionBtn.addEventListener('click', saveConnectionSettings);
  
  // 操作按钮
  clearHistoryBtn.addEventListener('click', clearChatHistory);
  resetSettingsBtn.addEventListener('click', resetAllSettings);
  saveAllBtn.addEventListener('click', saveAllSettings);
  
  // 日志弹窗
  viewLogsLink.addEventListener('click', (e) => {
    e.preventDefault();
    openLogsModal();
  });
  closeLogsBtn.addEventListener('click', closeLogsModal);
  clearLogsBtn.addEventListener('click', clearLogs);
  downloadLogsBtn.addEventListener('click', downloadLogs);
  
  // 点击弹窗外部关闭
  logsModal.addEventListener('click', (e) => {
    if (e.target === logsModal) {
      closeLogsModal();
    }
  });
}

// 从存储加载设置
async function loadSettings() {
  try {
    // 加载连接设置
    const connectionResult = await chrome.runtime.sendMessage({ type: 'GET_SETTINGS' });
    if (connectionResult.success) {
      const { serverUrl, apiKey, settings } = connectionResult.data;
      serverUrlInput.value = serverUrl || defaultSettings.serverUrl;
      apiKeyInput.value = apiKey || defaultSettings.apiKey;
      autoConnectCheckbox.checked = settings?.autoConnect ?? defaultSettings.autoConnect;
    }
    
    // 从同步存储加载其他设置
    const result = await chrome.storage.sync.get('picoclaw_settings');
    const settings = result.picoclaw_settings || {};
    
    showNotificationsCheckbox.checked = settings.showNotifications ?? defaultSettings.showNotifications;
    confirmActionsCheckbox.checked = settings.confirmActions ?? defaultSettings.confirmActions;
    highlightElementsCheckbox.checked = settings.highlightElements ?? defaultSettings.highlightElements;
    maxHistoryInput.value = settings.maxHistory ?? defaultSettings.maxHistory;
    sendPageInfoCheckbox.checked = settings.sendPageInfo ?? defaultSettings.sendPageInfo;
    excludePrivateCheckbox.checked = settings.excludePrivate ?? defaultSettings.excludePrivate;
    
  } catch (error) {
    console.error('加载设置失败:', error);
    showConnectionResult('加载设置失败: ' + error.message, 'error');
  }
}

// 保存连接设置
async function saveConnectionSettings() {
  const serverUrl = serverUrlInput.value.trim();
  const apiKey = apiKeyInput.value.trim();
  const autoConnect = autoConnectCheckbox.checked;
  
  if (!serverUrl) {
    showConnectionResult('请输入服务器地址', 'error');
    return;
  }
  
  try {
    const response = await chrome.runtime.sendMessage({
      type: 'SAVE_SETTINGS',
      data: {
        serverUrl,
        apiKey,
        settings: { autoConnect }
      }
    });
    
    if (response.success) {
      showConnectionResult('连接设置已保存', 'success');
    } else {
      showConnectionResult('保存失败: ' + response.error, 'error');
    }
  } catch (error) {
    showConnectionResult('保存失败: ' + error.message, 'error');
  }
}

// 测试连接
async function testConnection() {
  const serverUrl = serverUrlInput.value.trim();
  
  if (!serverUrl) {
    showConnectionResult('请输入服务器地址', 'error');
    return;
  }
  
  testConnectionBtn.disabled = true;
  testConnectionBtn.textContent = '测试中...';
  
  try {
    // 临时保存地址用于测试
    await chrome.runtime.sendMessage({
      type: 'SAVE_SETTINGS',
      data: { serverUrl }
    });
    
    const response = await chrome.runtime.sendMessage({ type: 'CHECK_CONNECTION' });
    
    if (response.connected) {
      showConnectionResult('✅ 连接成功！服务器正常运行', 'success');
    } else {
      showConnectionResult('❌ 连接失败，请检查服务器地址是否正确', 'error');
    }
  } catch (error) {
    showConnectionResult('❌ 连接失败: ' + error.message, 'error');
  } finally {
    testConnectionBtn.disabled = false;
    testConnectionBtn.textContent = '测试连接';
  }
}

// 保存所有设置
async function saveAllSettings() {
  try {
    // 保存连接设置
    await saveConnectionSettings();
    
    // 保存通用设置
    const settings = {
      showNotifications: showNotificationsCheckbox.checked,
      confirmActions: confirmActionsCheckbox.checked,
      highlightElements: highlightElementsCheckbox.checked,
      maxHistory: parseInt(maxHistoryInput.value) || 50,
      sendPageInfo: sendPageInfoCheckbox.checked,
      excludePrivate: excludePrivateCheckbox.checked
    };
    
    await chrome.storage.sync.set({ picoclaw_settings: settings });
    
    // 显示成功通知
    showNotification('所有设置已保存', 'success');
    
  } catch (error) {
    showNotification('保存失败: ' + error.message, 'error');
  }
}

// 清空对话历史
async function clearChatHistory() {
  if (!confirm('确定要清空所有对话历史吗？此操作不可恢复。')) {
    return;
  }
  
  try {
    await chrome.storage.local.remove('picoclaw_chat_history');
    showNotification('对话历史已清空', 'success');
  } catch (error) {
    showNotification('清空失败: ' + error.message, 'error');
  }
}

// 恢复默认设置
async function resetAllSettings() {
  if (!confirm('确定要恢复默认设置吗？所有自定义设置将丢失。')) {
    return;
  }
  
  try {
    // 重置连接设置
    await chrome.runtime.sendMessage({
      type: 'SAVE_SETTINGS',
      data: {
        serverUrl: defaultSettings.serverUrl,
        apiKey: defaultSettings.apiKey,
        settings: {
          autoConnect: defaultSettings.autoConnect
        }
      }
    });
    
    // 重置通用设置
    await chrome.storage.sync.set({ picoclaw_settings: defaultSettings });
    
    // 重新加载设置
    await loadSettings();
    
    showNotification('已恢复默认设置', 'success');
    
  } catch (error) {
    showNotification('重置失败: ' + error.message, 'error');
  }
}

// 显示连接结果
function showConnectionResult(message, type) {
  connectionResult.textContent = message;
  connectionResult.className = 'result-message ' + type;
  
  // 5 秒后自动隐藏
  setTimeout(() => {
    connectionResult.className = 'result-message';
  }, 5000);
}

// 显示通知
function showNotification(message, type = 'info') {
  // 创建通知元素
  const notification = document.createElement('div');
  notification.className = `notification ${type}`;
  notification.textContent = message;
  
  // 样式
  notification.style.cssText = `
    position: fixed;
    top: 20px;
    right: 20px;
    padding: 12px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    z-index: 9999;
    animation: slideIn 0.3s ease;
  `;
  
  if (type === 'success') {
    notification.style.background = '#48bb78';
    notification.style.color = 'white';
  } else if (type === 'error') {
    notification.style.background = '#f56565';
    notification.style.color = 'white';
  }
  
  document.body.appendChild(notification);
  
  // 3 秒后移除
  setTimeout(() => {
    notification.style.animation = 'slideOut 0.3s ease';
    setTimeout(() => notification.remove(), 300);
  }, 3000);
}

// 打开日志弹窗
async function openLogsModal() {
  logsModal.classList.add('show');
  await loadLogs();
}

// 关闭日志弹窗
function closeLogsModal() {
  logsModal.classList.remove('show');
}

// 加载日志
async function loadLogs() {
  try {
    // 获取扩展后台页面的日志
    // 实际实现中可能需要将日志存储在 storage 中
    logsContent.textContent = '日志功能开发中...\n\n' +
      '当前时间: ' + new Date().toLocaleString() + '\n' +
      '扩展版本: 1.0.0\n' +
      '浏览器: ' + navigator.userAgent + '\n';
  } catch (error) {
    logsContent.textContent = '加载日志失败: ' + error.message;
  }
}

// 清空日志
async function clearLogs() {
  if (!confirm('确定要清空日志吗？')) {
    return;
  }
  
  logsContent.textContent = '日志已清空\n';
}

// 下载日志
function downloadLogs() {
  const logs = logsContent.textContent;
  const blob = new Blob([logs], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  
  const a = document.createElement('a');
  a.href = url;
  a.download = `picoclaw-logs-${new Date().toISOString().slice(0, 10)}.txt`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  
  URL.revokeObjectURL(url);
}

// 添加动画样式
const style = document.createElement('style');
style.textContent = `
  @keyframes slideIn {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  
  @keyframes slideOut {
    from { transform: translateX(0); opacity: 1; }
    to { transform: translateX(100%); opacity: 0; }
  }
`;
document.head.appendChild(style);

// DOM 就绪后初始化
document.addEventListener('DOMContentLoaded', init);

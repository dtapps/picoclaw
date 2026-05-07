# PicoClaw 浏览器助手

PicoClaw 浏览器扩展，允许你通过 AI 助手控制浏览器、自动化网页操作。

## 功能特性

- 💬 **AI 聊天** - 与 PicoClaw AI 助手对话
- 🖱️ **浏览器控制** - 点击按钮、填写表单、滚动页面等
- 📄 **页面分析** - 获取当前页面的元素信息
- 🔧 **自定义设置** - 配置服务器地址、API Key 等

## 安装方法

### 开发者模式安装

1. 打开 Chrome/Edge 浏览器，进入扩展管理页面
   - Chrome: `chrome://extensions/`
   - Edge: `edge://extensions/`

2. 开启右上角的"开发者模式"

3. 点击"加载已解压的扩展程序"

4. 选择 `extensions/browser` 文件夹

5. 扩展已成功安装！

## 使用方法

1. 点击浏览器工具栏上的 PicoClaw 图标 (🦞)

2. 配置 PicoClaw 服务器地址：
   - 点击设置按钮 (⚙️)
   - 输入你的 PicoClaw Gateway 地址（默认 `http://localhost:18790`）
   - 点击"保存"

3. 开始聊天！你可以：
   - 询问问题
   - 让 AI 帮你点击页面元素
   - 让 AI 帮你填写表单
   - 让 AI 帮你滚动页面

## 示例指令

```
"点击登录按钮"
"在搜索框中输入 'PicoClaw'"
"滚动到页面底部"
"获取当前页面的所有链接"
"帮我填写注册表单"
```

## 支持的浏览器操作

- ✅ 点击元素
- ✅ 输入文本
- ✅ 滚动页面
- ✅ 导航到新页面
- ✅ 刷新页面
- ✅ 前进/后退
- ✅ 获取页面文本/HTML
- ✅ 填写表单
- ✅ 选择下拉选项
- ✅ 悬停/聚焦元素
- ✅ 清除输入框

## 项目结构

```
extensions/browser/
├── manifest.json          # 扩展配置
├── src/
│   ├── background/        # 后台服务脚本
│   │   └── background.js
│   ├── content/           # 内容脚本（注入页面）
│   │   └── content.js
│   ├── popup/             # 弹出窗口
│   │   ├── popup.html
│   │   ├── popup.css
│   │   └── popup.js
│   ├── options/           # 设置页面
│   │   ├── options.html
│   │   ├── options.css
│   │   └── options.js
│   └── common/            # 共享代码
│       ├── config.js
│       └── api-client.js
└── icons/                 # 图标文件
```

## 开发

### 修改代码后

扩展会自动重新加载（Manifest V3 支持）。如果没有自动刷新，点击扩展管理页面的刷新按钮。

### 调试

- **Popup**: 右键点击扩展图标 -> "检查弹出内容"
- **Background**: 扩展管理页面 -> 点击"服务工作进程"旁边的链接
- **Content Script**: 在普通网页中按 F12 打开开发者工具

## 配置说明

### 服务器设置

- **服务器地址**: PicoClaw 服务器的 URL
- **API Key**: 如果服务器需要认证
- **自动连接**: 启动浏览器时自动连接

### 通用设置

- **显示通知**: 执行操作时显示浏览器通知
- **执行操作前确认**: 敏感操作前请求确认
- **高亮操作元素**: 高亮显示正在操作的元素
- **最大历史消息数**: 保留的对话历史数量

### 隐私设置

- **发送页面信息**: 允许发送页面信息给 AI
- **排除隐私页面**: 不在敏感页面（银行、支付等）上激活

## 技术栈

- Manifest V3
- Vanilla JavaScript
- Chrome Extension APIs
- WebSocket (可选，用于实时通信)

## 许可证

MIT License - 与 PicoClaw 项目保持一致

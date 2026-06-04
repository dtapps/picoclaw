# 开发板引脚与引脚复用参考

## LicheeRV Nano (SG2002)

### I2C 总线

| 总线 | 引脚 | 说明 |
|-----|------|-------|
| I2C-1 | P18 (SCL), P21 (SDA) | **与 WiFi SDIO 共用** — 必须先关闭 WiFi |
| I2C-3 | 排针上可用 | 查看设备树确认引脚分配 |
| I2C-5 | 软件模拟 (BitBang) | 较慢，但无引脚冲突 |

### SPI 总线

| 总线 | 引脚 | 说明 |
|-----|------|-------|
| SPI-2 | P18 (CS), P21 (MISO), P22 (MOSI), P23 (SCK) | **与 WiFi 共用** — 必须先关闭 WiFi |
| SPI-4 | 软件模拟 (BitBang) | 较慢，但无引脚冲突 |

### I2C-1 配置步骤

```bash
# 1. 关闭 WiFi（与 I2C-1 共用引脚）
/etc/init.d/S30wifi stop

# 2. 配置 I2C-1 的引脚复用

devmem 0x030010D0 b 0x2   # P18 → I2C1_SCL
devmem 0x030010DC b 0x2   # P21 → I2C1_SDA

# 3. 加载 i2c-dev 模块
modprobe i2c-dev

# 4. 验证
ls /dev/i2c-*
```

### SPI-2 配置步骤

```bash
# 1. 关闭 WiFi（与 SPI-2 共用引脚）
/etc/init.d/S30wifi stop

# 2. 配置 SPI-2 的引脚复用

devmem 0x030010D0 b 0x1   # P18 → SPI2_CS
devmem 0x030010DC b 0x1   # P21 → SPI2_MISO
devmem 0x030010E0 b 0x1   # P22 → SPI2_MOSI
devmem 0x030010E4 b 0x1   # P23 → SPI2_SCK

# 3. 验证
ls /dev/spidev*
```

### 最大测试 SPI 速率
- SPI-2 硬件：已测试至 **93 MHz**
- `spidev_test` 已预装在官方镜像中，可用于回环测试

---

## MaixCAM

### I2C 总线

| 总线 | 引脚 | 说明 |
|-----|------|-------|
| I2C-1 | 与 WiFi 重叠 | 不推荐 |
| I2C-3 | 与 WiFi 重叠 | 不推荐 |
| I2C-5 | A15 (SCL), A27 (SDA) | **推荐** — 软件 I2C，无冲突 |

### I2C-5 配置步骤

```bash
# 使用 pinmap 工具配置引脚
# （MaixCAM 使用 pinmap 工具代替 devmem）
# 参考：https://wiki.sipeed.com/hardware/en/maixcam/gpio.html

# 加载 i2c-dev
modprobe i2c-dev

# 验证
ls /dev/i2c-*
```

---

## MaixCAM2

### I2C 总线

| 总线 | 引脚 | 说明 |
|-----|------|-------|
| I2C-6 | A1 (SCL), A0 (SDA) | 排针上可用 |
| I2C-7 | 可用 | 查看设备树 |

### 配置步骤

```bash
# 配置 pinmap 以使用 I2C-6
# A1 → I2C6_SCL, A0 → I2C6_SDA
# 参考 MaixCAM2 文档了解 pinmap 命令

modprobe i2c-dev
ls /dev/i2c-*
```

---

## NanoKVM

采用与 LicheeRV Nano 相同的 SG2002 SoC。GPIO 和 I2C 访问遵循相同的引脚复用流程。参考上方的 LicheeRV Nano 章节。

查看 NanoKVM 专用排针上可用的 I2C/SPI 线路：
- https://wiki.sipeed.com/hardware/en/kvm/NanoKVM/introduction.html

---

## 常见问题

### 找不到 devmem
`devmem` 工具可能不在默认镜像中。解决方案：
- 如果已安装 busybox，使用 `busybox devmem`
- 从 Sipeed 软件源下载 devmem
- 从源码交叉编译（单个 C 文件）

### 动态总线编号
I2C 适配器编号可能因驱动加载顺序而在每次启动时变化。始终使用 `i2c detect` 查找当前总线分配，而不是硬编码总线编号。

### 权限
`/dev/i2c-*` 和 `/dev/spidev*` 通常需要 root 权限。解决方案：
- 以 root 身份运行 picoclaw
- 将用户加入 `i2c` 和 `spi` 组
- 创建 udev 规则：`SUBSYSTEM=="i2c-dev", MODE="0666"`

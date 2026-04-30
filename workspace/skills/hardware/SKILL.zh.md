---
name: hardware
description: 在 Sipeed 开发板（LicheeRV Nano、MaixCAM、NanoKVM）上读写和控制 I2C 和 SPI 外设。
homepage: https://wiki.sipeed.com/hardware/en/lichee/RV_Nano/1_intro.html
metadata: {"nanobot":{"emoji":"🔧","requires":{"tools":["i2c","spi"]}}}
---

# 硬件 (I2C / SPI)

使用 `i2c` 和 `spi` 工具与连接到开发板的传感器、显示器和其他外设进行交互。

## 快速开始

```
# 1. 查找可用总线
i2c detect

# 2. 扫描连接的设备
i2c scan  (bus: "1")

# 3. 从传感器读取数据（例如 AHT20 温湿度传感器）
i2c read  (bus: "1", address: 0x38, register: 0xAC, length: 6)

# 4. SPI 设备
spi list
spi read  (device: "2.0", length: 4)
```

## 开始之前 —— Pinmux 设置

大多数 I2C/SPI 引脚在 Sipeed 开发板上与 WiFi 共享。使用前必须配置 pinmux。

有关开发板特定的命令，请参阅 `references/board-pinout.md`。

**常见步骤：**
1. 如果使用共享引脚，停止 WiFi：`/etc/init.d/S30wifi stop`
2. 加载 i2c-dev 模块：`modprobe i2c-dev`
3. 使用 `devmem` 配置 pinmux（开发板特定）
4. 使用 `i2c detect` 和 `i2c scan` 验证

## 安全

- **写操作** 需要 `confirm: true` —— 始终先与用户确认
- I2C 地址验证为 7 位范围（0x03-0x77）
- SPI 模式验证（仅 0-3）
- 每笔交易最大：256 字节（I2C），4096 字节（SPI）

## 常见设备

有关流行传感器的寄存器映射和使用方法，请参阅 `references/common-devices.md`：
AHT20、BME280、SSD1306 OLED、MPU6050 IMU、DS3231 RTC、INA219 电源监控器、PCA9685 PWM 等。

## 故障排除

| 问题 | 解决方案 |
|---------|----------|
| 找不到 I2C 总线 | `modprobe i2c-dev` 并检查设备树 |
| 权限被拒绝 | 以 root 身份运行或将用户添加到 `i2c` 组 |
| 扫描不到设备 | 检查接线、上拉电阻（典型值 4.7k）和 pinmux |
| 总线编号改变 | I2C 适配器编号可能在启动之间变化；使用 `i2c detect` 查找当前分配 |
| WiFi 停止工作 | I2C-1/SPI-2 与 WiFi SDIO 共享引脚；不能同时使用两者 |
| 找不到 `devmem` | 单独下载或使用 `busybox devmem` |
| SPI 传输返回全零 | 检查 MISO 接线和设备电源 |
| SPI 传输返回全 0xFF | 设备无响应；检查 CS 引脚和时钟极性（模式） |

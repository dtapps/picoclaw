---
name: weather
description: 获取当前天气和预报，并验证位置匹配（无需 API 密钥）。
homepage: https://wttr.in/:help
metadata: {"nanobot":{"emoji":"🌤️","requires":{"bins":["curl"]}}}
---

# 天气

首先使用最可靠的位置匹配。对于中文城市名称或其他非拉丁输入，优先使用 `wttr.in` 并保留原始查询，因为它直接解析本地名称。只有在确认确切城市后，才使用 Open-Meteo 获取结构化的当前状况和预报。

## 准确性规则

- 始终在最终答案中重述匹配的位置、地区/国家和观测时间。
- 不要盲目相信第一个地理编码结果。检查 `country`、`admin1`、`admin2` 和 `population`。
- 对于中文城市查询，除非顶部结果明显正确，否则不要将汉字直接发送到 Open-Meteo 地理编码。优先使用 `wttr.in` 并保留原始中文名称，或者使用英文/拼音城市名称进行地理编码。
- 如果仍有多个可能匹配，请提出后续问题或明确说明假设。
- 调用 Open-Meteo 时使用 `timezone=auto`，以便报告的时间与位置匹配。

## wttr.in（最适合直接城市名称查询）

快速获取当前状况：
```bash
curl -s "https://wttr.in/London?format=%l:+%c+%t+%h+%w"
```

中文城市示例：
```bash
curl -s "https://wttr.in/%E6%88%90%E9%83%BD?format=%l:+%c+%t+%h+%w"
curl -s "https://wttr.in/%E4%B8%8A%E6%B5%B7?format=%l:+%c+%t+%h+%w"
```

如果需要更多详细信息，使用 JSON 输出：
```bash
curl -s "https://wttr.in/Chengdu?format=j1"
```

提示：
- URL 编码空格：`New York` -> `New+York`
- 发送请求前对非 ASCII 文本进行 URL 编码
- 使用 `?m` 表示公制单位，`?u` 表示美制单位

## Open-Meteo（最适合结构化预报）

1. 对城市进行地理编码并验证返回的位置元数据：
```bash
curl -s "https://geocoding-api.open-meteo.com/v1/search?name=Chengdu&count=3&language=en&format=json"
```

2. 使用验证后的坐标查询当前天气和今日预报：
```bash
curl -s "https://api.open-meteo.com/v1/forecast?latitude=30.66667&longitude=104.06667&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m&daily=weather_code,temperature_2m_max,temperature_2m_min&forecast_days=1&timezone=auto"
```

重要提示：
- 对于 `成都` 等中文输入，地理编码 `name=%E6%88%90%E9%83%BD` 可能会首先返回较小的同名位置。在验证其匹配中国四川后，优先使用 `Chengdu`。
- 如果地理编码看起来可疑，请回退到 `wttr.in` 并使用原始城市名称，而不是呈现可能错误的结果。

文档：https://open-meteo.com/en/docs

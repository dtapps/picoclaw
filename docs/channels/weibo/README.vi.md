> Quay lại [README](../../../README.vi.md)

# Kênh Weibo

PicoClaw hỗ trợ kết nối đến Weibo như một kênh sử dụng API chính thức của Weibo qua WebSocket.

## Những gì kênh này hỗ trợ

- Nhận và gửi tin nhắn trực tiếp qua Weibo
- Giao tiếp thời gian thực dựa trên WebSocket
- Xử lý tin nhắn văn bản
- Lọc danh sách cho phép người gửi
- Định tuyến đầu ra suy luận đến một cuộc trò chuyện riêng biệt

> Không cần URL webhook công khai. PicoClaw thiết lập kết nối WebSocket gửi ra đến máy chủ Weibo.

---

## Bắt Đầu Nhanh

### Lấy Thông Tin Xác Thực

1. Mở ứng dụng Weibo (di động hoặc web)
2. Gửi tin nhắn trực tiếp đến **@微博龙虾助手**
3. Gửi tin nhắn: **连接龙虾**
4. Bạn sẽ nhận được phản hồi với thông tin xác thực:

```
您的应用凭证信息如下：

AppId: your-app-id
AppSecret: your-app-secret
```

> Để đặt lại thông tin xác thực, gửi tin nhắn "重置凭证".

### Cấu Hình

Thêm phần sau vào `config.json` của bạn dưới `channels`:

```json
{
  "channels": {
    "weibo": {
      "enabled": true,
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": [],
      "reasoning_channel_id": ""
    }
  }
}
```

Sau đó khởi động gateway:

```bash
picoclaw gateway
```

---

## Cấu Hình

| Trường | Loại | Mặc Định | Mô Tả |
| ----- | ---- | ------- | ----------- |
| `enabled` | bool | `false` | Bật kênh Weibo. |
| `app_id` | string | — | App ID của ứng dụng Weibo của bạn. Bắt buộc khi bật. |
| `app_secret` | string | — | App Secret của ứng dụng Weibo. Được lưu trữ mã hóa trong `.security.yml`. Bắt buộc khi bật. |
| `allow_from` | array | `[]` | Danh sách cho phép người gửi. Trống có nghĩa là cho phép tất cả. |
| `reasoning_channel_id` | string | `""` | ID cuộc trò chuyện tùy chọn để định tuyến đầu ra suy luận/suy nghĩ đến một cuộc trò chuyện riêng biệt. |

### Biến Môi Trường

Tất cả các trường có thể được ghi đè qua biến môi trường với tiền tố `PICOCLAW_CHANNELS_WEIBO_`:

| Biến Môi Trường | Trường Tương Ứng |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_WEIBO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_WEIBO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_WEIBO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_WEIBO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_WEIBO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Hành Vi Runtime

- PicoClaw duy trì kết nối WebSocket hoạt động đến máy chủ Weibo.
- Tin nhắn văn bản đến được xử lý bởi agent và phản hồi được gửi qua API Weibo.
- Phương tiện đến được tải xuống bộ nhớ phương tiện cục bộ trước khi chuyển đến agent.
- Tin nhắn trùng lặp được phát hiện và ngăn chặn.

---

## Khắc Phục Sự Cố

### Kết nối thất bại

- Xác minh `app_id` và `app_secret` đúng.
- Đảm bảo tài khoản Weibo của bạn đã được ủy quyền.
- Kiểm tra máy chủ của bạn có thể kết nối đến endpoint WebSocket của Weibo.

### Tin nhắn không đến

- Kiểm tra `allow_from` có đang chặn người gửi không.
- Đảm bảo `channels.weibo.enabled` được đặt thành `true`.
- Xác minh `app_id` và `app_secret` không trống.

### Cần đặt lại thông tin xác thực

- Gửi tin nhắn "重置凭证" đến @微博龙虾助手 để lấy thông tin xác thực mới.

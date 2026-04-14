> Quay lại [README](../../../README.vi.md)

# Kênh Tencent Yuanbao

PicoClaw hỗ trợ kết nối đến Tencent Yuanbao như một kênh sử dụng API chính thức của Yuanbao Bot qua WebSocket.

## Những gì kênh này hỗ trợ

- Gửi và nhận tin nhắn trực tiếp và nhóm
- Giao tiếp thời gian thực dựa trên WebSocket với Yuanbao
- Xử lý tin nhắn văn bản
- Cấu hình kích hoạt nhóm (chế độ chỉ @mention)
- Lọc danh sách cho phép người gửi
- Định tuyến đầu ra suy luận đến một cuộc trò chuyện riêng biệt

> Không cần URL webhook công khai. PicoClaw thiết lập kết nối WebSocket gửi ra đến máy chủ Yuanbao.

---

## Bắt Đầu Nhanh

### Điều Kiện Tiên Quyết

Bạn cần lấy thông tin xác thực cho bot Yuanbao của mình:
- **App ID** (`app_id`)
- **App Secret** (`app_secret`)

### Cấu Hình

Thêm phần sau vào `config.json` của bạn dưới `channel_list`:

```json
{
  "channel_list": {
    "yuanbao": {
      "enabled": true,
      "type": "yuanbao",
      "app_id": "YOUR_APP_ID",
      "app_secret": "YOUR_APP_SECRET",
      "allow_from": [],
      "group_trigger": {},
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
| `enabled` | bool | `false` | Bật kênh Yuanbao. |
| `app_id` | string | — | App ID của ứng dụng Yuanbao của bạn. Bắt buộc khi bật. |
| `app_secret` | string | — | App Secret của ứng dụng Yuanbao. Được lưu trữ mã hóa trong `.security.yml`. Bắt buộc khi bật. |
| `allow_from` | array | `[]` | Danh sách cho phép người gửi. Trống có nghĩa là cho phép tất cả. |
| `group_trigger` | object | `{}` | Cài đặt kích hoạt nhóm. |
| `reasoning_channel_id` | string | `""` | ID cuộc trò chuyện tùy chọn để định tuyến đầu ra suy luận/suy nghĩ đến một cuộc trò chuyện riêng biệt. |

### Cấu Hình Kích Hoạt Nhóm

```json
{
  "group_trigger": {
    "mention_only": true
  }
}
```

| Trường | Loại | Mặc Định | Mô Tả |
| ----- | ---- | ------- | ----------- |
| `mention_only` | bool | `true` | Khi true, bot chỉ phản hồi khi được @mention trong nhóm chat. |

### Biến Môi Trường

Tất cả các trường có thể được ghi đè qua biến môi trường với tiền tố `PICOCLAW_CHANNELS_YUANBAO_`:

| Biến Môi Trường | Trường Tương Ứng |
| -------------------- | ------------------- |
| `PICOCLAW_CHANNELS_YUANBAO_ENABLED` | `enabled` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_ID` | `app_id` |
| `PICOCLAW_CHANNELS_YUANBAO_APP_SECRET` | `app_secret` |
| `PICOCLAW_CHANNELS_YUANBAO_ALLOW_FROM` | `allow_from` |
| `PICOCLAW_CHANNELS_YUANBAO_REASONING_CHANNEL_ID` | `reasoning_channel_id` |

---

## Hành Vi Runtime

- PicoClaw duy trì kết nối WebSocket hoạt động đến máy chủ Yuanbao.
- Tin nhắn văn bản đến được xử lý bởi agent và phản hồi được gửi qua API Yuanbao.
- Tin nhắn trực tiếp được gửi trực tiếp đến người dùng.
- Tin nhắn nhóm được gửi đến chat nhóm.
- Tin nhắn trùng lặp được phát hiện và ngăn chặn.

---

## Khắc Phục Sự Cố

### Kết nối thất bại

- Xác minh `app_id` và `app_secret` đúng.
- Đảm bảo ứng dụng Yuanbao của bạn đã được kích hoạt các quyền cần thiết.
- Kiểm tra máy chủ của bạn có thể kết nối đến endpoint WebSocket của Yuanbao.

### Tin nhắn không đến

- Kiểm tra `allow_from` có đang chặn người gửi không.
- Đảm bảo `channels.yuanbao.enabled` được đặt thành `true`.
- Xác minh `app_id` và `app_secret` không trống.
- Đối với nhóm chat, đảm bảo `group_trigger.mention_only` được cấu hình đúng.

---
name: stock-recommendation-skill
description: Bộ skill phân tích chứng khoán Việt Nam theo mô hình AI Trading Team. Triggers on tasks involving phân tích thị trường, quản lý rủi ro, và quyết định đầu tư.
---

# VN Stock Analysis Skill

## Mô tả

Bộ skill phân tích chứng khoán Việt Nam theo mô hình AI Trading Team gồm 3 chuyên gia:

- **Nguyễn Minh Anh** — Chuyên gia phân tích thị trường
- **Trần Quốc Bảo** — Chuyên gia quản lý rủi ro
- **Lê Thị Mai** — Giám đốc đầu tư (quyết định cuối)

## Cách dùng

Paste prompt sau vào Claude bất cứ lúc nào:

```
Chạy VN Stock Analysis cho [MÃ CỔ PHIẾU].
Số tiền đầu tư: [X] triệu VND
Mức độ rủi ro: [thấp / trung bình / cao]
Thời gian nắm giữ: [X tháng]
```

Ví dụ:

```
Chạy VN Stock Analysis cho VCB.
Số tiền đầu tư: 100 triệu VND
Mức độ rủi ro: trung bình
Thời gian nắm giữ: 3 tháng
```

## Luồng phân tích

```
Bước 1: Minh Anh — Phân tích kỹ thuật + cơ bản
    ↓
Bước 2: Quốc Bảo — Đánh giá rủi ro + phản biện Minh Anh
    ↓
Bước 3: Thị Mai — Tổng hợp + quyết định cuối cùng
    ↓
Output: MUA / BÁN / GIỮ + entry zone + stop loss + take profit
```

## Quy tắc áp dụng

- Đọc `rules/market-researcher.md` để hiểu cách Minh Anh phân tích
- Đọc `rules/risk-analysis.md` để hiểu cách Quốc Bảo đánh giá
- Đọc `rules/investor.md` để hiểu cách Thị Mai quyết định

## Lưu ý quan trọng

- Luôn search web để lấy giá và tin tức mới nhất trước khi phân tích
- Mỗi chuyên gia phải đọc và phản hồi lại ý kiến của người trước
- Output cuối phải có con số cụ thể: giá vào, stop loss, TP1, TP2
- Đây là công cụ hỗ trợ quyết định, không phải lời khuyên tài chính tuyệt đối

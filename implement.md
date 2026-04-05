# Stock Monitor Bot — Implementation Plan

## Mục tiêu

Bot chạy trên Android cũ (Termux), theo dõi giá VNM/ACB/HPG mỗi 5 phút,
gửi alert qua Telegram khi giá vào vùng mua hoặc xuyên stop loss.
Không tự động đặt lệnh — chỉ alert để bạn quyết định.

---

## Tech stack

| Thành phần     | Lựa chọn                          | Lý do                              |
|----------------|------------------------------------|------------------------------------|
| Ngôn ngữ       | Go 1.22+                          | Binary nhẹ, chạy tốt trên ARM     |
| Scheduler      | `go-co-op/gocron/v2`              | Cron đơn giản, lightweight         |
| HTTP client    | `net/http` (stdlib)               | Không cần thêm dependency          |
| Telegram       | `go-telegram-bot-api/telegram-bot-api/v5` | Mature, stable            |
| Storage        | `go.etcd.io/bbolt`                | Embedded KV, không cần server      |
| Config         | `gopkg.in/yaml.v3`               | Đọc watchlist + strategy           |
| Data source    | SSI API hoặc cafef scraping       | Giá realtime miễn phí              |
| Platform       | Termux trên Android               | Zero cost, chạy 24/7               |

---

## Cấu trúc project

```
stock-bot/
├── main.go                 # Entry point, khởi tạo scheduler
├── config/
│   ├── config.go           # Load + parse YAML config
│   ├── watchlist.yaml      # Danh sách mã + vùng giá
│   └── strategy.yaml       # Chiến lược DCA + điều kiện
├── provider/
│   ├── provider.go         # Interface lấy giá
│   ├── ssi.go              # Impl: gọi SSI API
│   └── cafef.go            # Impl: scrape cafef (backup)
├── engine/
│   ├── checker.go          # So sánh giá vs vùng mua
│   ├── signal.go           # Tạo signal BUY/SELL/WATCH
│   └── dca.go              # Theo dõi đợt DCA đã mua chưa
├── alert/
│   ├── telegram.go         # Gửi message qua Telegram Bot API
│   └── formatter.go        # Format message đẹp cho Telegram
├── store/
│   └── bolt.go             # Lưu trạng thái DCA, lịch sử giá
├── cmd/
│   └── tgcommand.go        # Xử lý /status /portfolio /bought
├── go.mod
├── go.sum
└── .env                    # TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID
```

---

## Config files

### watchlist.yaml

```yaml
stocks:
  - code: VNM
    name: Vinamilk
    exchange: HOSE
    buy_good:          # Vùng mua tốt
      low: 56000
      high: 58000
    buy_great:         # Vùng mua tuyệt vời
      low: 52000
      high: 54000
    stop_loss: 53000
    tp1: 68000
    tp2: 75000

  - code: ACB
    name: Ngân hàng Á Châu
    exchange: HOSE
    buy_good:
      low: 21500
      high: 22500
    buy_great:
      low: 20000
      high: 21000
    stop_loss: 20500
    tp1: 28000
    tp2: 31000

  - code: HPG
    name: Hòa Phát
    exchange: HOSE
    buy_good:
      low: 24500
      high: 25500
    buy_great:
      low: 22000
      high: 23500
    stop_loss: 23000
    tp1: 32000
    tp2: 36000
```

### strategy.yaml

```yaml
portfolio:
  total_capital: 13000000     # 13 triệu (10 mới + 3 từ OIL)
  allocation:
    VNM: 0.30                 # 30% = 3.9 triệu
    ACB: 0.35                 # 35% = 4.55 triệu
    HPG: 0.35                 # 35% = 4.55 triệu

dca:
  rounds: 3                   # 3 đợt mua
  interval_days: 10           # Cách nhau 10 ngày

market_conditions:
  vnindex_floor: 1645         # Không mua nếu VN-Index dưới ngưỡng này
  oil_price_ceiling: 150      # Cảnh báo nếu dầu vượt 150 USD

schedule:
  check_interval: "*/5 9-15 * * 1-5"   # Mỗi 5 phút, 9h-15h, T2-T6
  daily_report: "0 20 * * 1-5"          # Report lúc 8PM T2-T6
  weekend_summary: "0 10 * * 6"         # Tóm tắt tuần lúc 10AM T7
```

---

## Modules chi tiết

### 1. provider — Lấy giá realtime

```
Interface:
  GetPrice(code string) → (price float64, change float64, volume int64, err error)
  GetVNIndex() → (points float64, change float64, err error)

Impl SSI:
  GET https://iboard.ssi.com.vn/dchart/api/1.1/defaultAllStocks
  Parse JSON → filter theo mã → trả về giá khớp lệnh gần nhất

Impl CafeF (backup):
  GET https://cafef.vn/du-lieu/hose/{code}
  Parse HTML → extract giá hiện tại
  Dùng khi SSI API lỗi

Retry logic:
  Max 3 lần, delay 5s giữa mỗi lần
  Nếu cả 2 provider fail → gửi alert "⚠️ Không lấy được giá"
```

### 2. engine/checker — Logic so sánh

```
Input:  giá hiện tại + config vùng giá
Output: Signal (enum)

Logic:
  price <= buy_great.high AND price >= buy_great.low
    → SIGNAL_BUY_GREAT ("Vùng mua tuyệt vời")

  price <= buy_good.high AND price >= buy_good.low
    → SIGNAL_BUY_GOOD ("Vùng mua tốt")

  price <= stop_loss
    → SIGNAL_STOP_LOSS ("Xuyên stop loss")

  price >= tp1 AND chưa chốt TP1
    → SIGNAL_TP1 ("Chạm TP1")

  price >= tp2 AND chưa chốt TP2
    → SIGNAL_TP2 ("Chạm TP2")

  else
    → SIGNAL_WATCH ("Theo dõi, chưa vào vùng")
```

### 3. engine/dca — Quản lý DCA

```
State (lưu trong BoltDB):
  {
    "VNM": {
      "rounds_bought": 1,
      "total_rounds": 3,
      "avg_price": 57500,
      "total_shares": 60,
      "total_invested": 3450000,
      "last_buy_date": "2026-04-21",
      "tp1_sold": false,
      "tp2_sold": false
    }
  }

Rules:
  - Không gửi signal BUY nếu rounds_bought >= total_rounds
  - Không gửi signal BUY nếu last_buy_date < interval_days trước
  - Không gửi signal BUY nếu VN-Index < vnindex_floor
  - Tính số tiền mua = allocation / total_rounds
  - Tính số cổ phiếu = số tiền / giá hiện tại (làm tròn xuống lô 100)
```

### 4. alert/telegram — Gửi alert

```
Telegram Bot API:
  POST https://api.telegram.org/bot{TOKEN}/sendMessage
  Body: { chat_id, text, parse_mode: "HTML" }

Message types:

  BUY SIGNAL:
  ────────────────────────
  🟢 <b>VNM VÀO VÙNG MUA TỐT</b>
  Giá: 57,200 VND
  Vùng: 56,000 - 58,000
  Đợt: 1/3 — Mua 3,900,000 VND (~68 cổ)
  VN-Index: 1,672 ✓
  ────────────────────────

  STOP LOSS:
  ────────────────────────
  🔴 <b>HPG XUYÊN STOP LOSS</b>
  Giá: 22,800 VND
  Stop loss: 23,000 VND
  Lỗ ước tính: -320,000 VND
  ⚡ Cân nhắc bán ngay phiên tới
  ────────────────────────

  DAILY REPORT (8PM):
  ────────────────────────
  📊 <b>BÁO CÁO NGÀY 21/04/2026</b>

  VNM  60,200 (+0.3%)  ⏳ Chờ 56-58K
  ACB  23,100 (-1.5%)  ⏳ Gần vùng mua
  HPG  26,400 (-0.9%)  ⏳ Chờ 24.5-25.5K

  VN-Index: 1,668 (+0.4%)
  Dầu Brent: 105.2 USD

  Trạng thái DCA:
  VNM: 0/3 đợt | ACB: 0/3 | HPG: 0/3

  💰 Tiền mặt còn lại: 13,000,000 VND
  ────────────────────────

  TP HIT:
  ────────────────────────
  🎯 <b>ACB CHẠM TP1</b>
  Giá: 28,100 VND
  TP1: 28,000 VND (+21% từ entry)
  Lãi: +950,000 VND
  👉 Cân nhắc bán 50% vị thế
  ────────────────────────
```

### 5. cmd/tgcommand — Xử lý lệnh Telegram

```
/status
  → Trả về giá 3 mã + khoảng cách đến vùng mua + VN-Index

/portfolio
  → Trả về danh mục đã mua, giá vốn, P&L hiện tại

/bought VNM 57200 60
  → Ghi nhận: đã mua 60 cổ VNM giá 57,200
  → Cập nhật DCA state (rounds_bought + 1)
  → Reply xác nhận + tính lại avg price

/adjust VNM buy_good 55000 57000
  → Cập nhật vùng giá mua trong runtime
  → Reply xác nhận vùng mới

/pause
  → Tạm dừng mọi alert (khi bạn đi du lịch chẳng hạn)

/resume
  → Bật lại alert

/help
  → Danh sách lệnh
```

### 6. store/bolt — Lưu trữ

```
BoltDB buckets:
  "dca_state"    → JSON trạng thái DCA mỗi mã
  "price_log"    → Giá mỗi 5 phút (giữ 30 ngày, tự xóa cũ)
  "signal_log"   → Lịch sử signal đã gửi (tránh gửi trùng)
  "config_cache" → Config runtime (sau /adjust)

Anti-spam rule:
  Không gửi cùng loại signal cho cùng mã trong 2 giờ
  Trừ STOP_LOSS — luôn gửi ngay
```

---

## Flow chính (main.go)

```
main()
  ├─ Load .env (token, chat_id)
  ├─ Load watchlist.yaml + strategy.yaml
  ├─ Init BoltDB
  ├─ Init Telegram bot (polling mode cho commands)
  ├─ Init scheduler
  │   ├─ Job "check_price": mỗi 5 phút (9h-15h T2-T6)
  │   │   ├─ Lấy giá 3 mã + VN-Index
  │   │   ├─ Chạy checker cho từng mã
  │   │   ├─ Chạy DCA logic
  │   │   ├─ Nếu có signal → format + gửi Telegram
  │   │   └─ Lưu price_log + signal_log
  │   ├─ Job "daily_report": 8PM T2-T6
  │   │   └─ Tổng hợp giá + P&L + trạng thái DCA → gửi
  │   └─ Job "weekend_summary": 10AM T7
  │       └─ Tổng kết tuần: giá thay đổi %, signal đã gửi
  └─ Start (blocking)
```

---

## Thứ tự implement

### Phase 1 — MVP (ngày 1-2)
Chạy được, gửi được alert cơ bản.

```
1. go mod init stock-bot
2. Viết config/config.go — load 2 file YAML
3. Viết provider/ssi.go — lấy giá 1 mã
4. Viết engine/checker.go — so sánh giá vs vùng
5. Viết alert/telegram.go — gửi 1 message text
6. Viết main.go — scheduler chạy mỗi 5 phút
7. Test trên máy tính trước
```

### Phase 2 — DCA + Storage (ngày 3-4)
Theo dõi trạng thái mua.

```
8.  Viết store/bolt.go — init DB, get/set state
9.  Viết engine/dca.go — logic đợt mua
10. Viết alert/formatter.go — format message đẹp
11. Thêm VN-Index check vào flow
12. Thêm anti-spam (không gửi trùng signal)
```

### Phase 3 — Telegram commands (ngày 5-6)
Tương tác 2 chiều.

```
13. Viết cmd/tgcommand.go — handler cho /status
14. Thêm /portfolio, /bought, /adjust
15. Thêm /pause, /resume
16. Daily report job
```

### Phase 4 — Deploy lên Android (ngày 7)
Chạy thật.

```
17. Cài Termux + Go trên Android
18. go build -o stock-bot
19. Tạo script khởi động + termux-wake-lock
20. Test chạy 1 ngày, verify alert đến Telegram
21. Tắt battery optimization cho Termux
```

---

## Lưu ý khi build trên Termux

```bash
# Cài Go
pkg install golang

# Clone project
git clone <your-repo> ~/stock-bot
cd ~/stock-bot

# Build trực tiếp trên điện thoại (quan trọng — không cross-compile)
go build -o stock-bot .

# Chạy
export TELEGRAM_BOT_TOKEN="xxx"
export TELEGRAM_CHAT_ID="xxx"
./stock-bot

# Giữ process sống khi tắt màn hình
termux-wake-lock

# Auto-restart nếu crash
while true; do ./stock-bot; echo "Crashed, restarting in 5s..."; sleep 5; done
```

---

## Test checklist trước khi chạy thật

```
[ ] Gọi API lấy được giá VNM, ACB, HPG
[ ] Gọi API lấy được VN-Index
[ ] Gửi được message Telegram
[ ] Signal BUY_GOOD trigger đúng khi giá trong vùng
[ ] Signal STOP_LOSS trigger đúng
[ ] Anti-spam: không gửi trùng trong 2 giờ
[ ] /status trả về đúng giá
[ ] /bought cập nhật đúng DCA state
[ ] Daily report gửi đúng 8PM
[ ] Chạy 1 ngày trên Termux không crash
[ ] Wake-lock hoạt động khi tắt màn hình
```

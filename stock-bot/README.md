# Stock Monitor Bot

Bot theo doi gia co phieu VNM/ACB/HPG, gui alert qua Telegram khi gia vao vung mua hoac xuyen stop loss.

## Yeu cau

- Go 1.22+
- Telegram Bot Token (tao qua [@BotFather](https://t.me/BotFather))
- Telegram Chat ID

## Cai dat

```bash
git clone <repo> && cd stock-bot
go mod tidy
cp .env.example .env   # hoac sua truc tiep file .env
```

Sua `.env`:

```
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_ID=your_chat_id_here
```

## Chay

```bash
# Chay truc tiep
make run

# Build binary roi chay
make build
./stock-bot
```

## Cau truc

```
config/         Load watchlist.yaml + strategy.yaml
provider/       Lay gia tu SSI (chinh) va CafeF (backup)
engine/         So sanh gia vs vung mua, logic DCA
alert/          Gui message qua Telegram Bot API
store/          Luu trang thai DCA, lich su gia (BoltDB)
cmd/            Xu ly lenh Telegram (/status, /bought, ...)
```

## Lenh Telegram

| Lenh | Mo ta |
|------|-------|
| `/status` | Gia hien tai + trang thai |
| `/portfolio` | Danh muc da mua + P&L |
| `/bought VNM 57200 60` | Ghi nhan da mua |
| `/adjust VNM buy_good 55000 57000` | Dieu chinh vung gia |
| `/pause` | Tam dung alert |
| `/resume` | Bat lai alert |
| `/help` | Danh sach lenh |

## Cau hinh

- `config/watchlist.yaml` — Ma co phieu + vung gia mua/ban
- `config/strategy.yaml` — Von, ty le phan bo, DCA, lich trinh

## Makefile

```
make run     Chay truc tiep (go run .)
make build   Build binary
make tidy    go mod tidy
make clean   Xoa binary + DB
```

# Chạy Go Server 24/7 trên Android via Termux + SSH

## Bước 1: Cài Termux

- Tải **Termux** từ F-Droid (khuyên dùng) hoặc CH Play
- Mở Termux, cài SSH server:

```bash
pkg update && pkg install openssh
```

- Set password:

```bash
passwd
```

- Khởi động SSH server:

```bash
sshd
```

- Lấy IP điện thoại:

```bash
ifconfig | grep inet
```

---

## Bước 2: SSH từ Mac vào điện thoại

```bash
ssh -p 8022 <username>@<IP_dien_thoai>

# Ví dụ:
ssh -p 8022 u0_a311@192.168.1.11
```

> Nhập password đã set ở bước 1.

---

## Bước 3: Cài Go trong Termux

```bash
pkg update && pkg install golang

# Kiểm tra
go version
```

---

## Bước 4: Copy code vào điện thoại (từ Mac)

```bash
# Push toàn bộ folder lên điện thoại
adb push . /data/local/tmp/

# Trong SSH session - copy vào Termux home
cp -r /data/local/tmp/stock-recommendation-skill ~/stock-bot
cd ~/stock-bot
```

---

## Bước 5: Build và chạy server

```bash
cd ~/stock-bot

# Cài dependencies
go mod tidy

# Chạy thử
go run .
```

---

## Bước 6: Chạy 24/7 với tmux

```bash
# Cài tmux
pkg install tmux

# Tạo session mới
tmux new -s stock-server

# Chạy server với auto-restart
while true; do
  go run .
  echo "Crashed! Restarting in 5s..."
  sleep 5
done
```

**Detach khỏi tmux** (server vẫn chạy ngầm):
```
Ctrl+B  rồi  D
```

**Reconnect lại sau:**
```bash
tmux attach -t stock-server
```

---

## Bước 7: Giữ server không bị kill

**Trong Termux:**
```bash
pkg install termux-api
termux-wake-lock
```

**Trên điện thoại (làm tay):**
```
Cài đặt → Pin → Battery Optimization
→ Tìm Termux → Chọn "Don't optimize"
```

---

## Tóm tắt lệnh hay dùng

| Việc | Lệnh |
|------|------|
| SSH vào điện thoại | `ssh -p 8022 <user>@<IP>` |
| Xem server đang chạy | `tmux attach -t stock-server` |
| Detach khỏi tmux | `Ctrl+B` rồi `D` |
| List tmux sessions | `tmux ls` |
| Push code mới từ Mac | `adb push . /data/local/tmp/` |

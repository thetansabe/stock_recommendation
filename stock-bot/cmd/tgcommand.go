package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"stock-bot/alert"
	"stock-bot/config"
	"stock-bot/provider"
	"stock-bot/store"
)

type CommandHandler struct {
	bot     *tgbotapi.BotAPI
	chatID  int64
	cfg     *config.AppConfig
	store   *store.Store
	alerter *alert.Alerter
	prov    provider.Provider
}

func NewCommandHandler(
	bot *tgbotapi.BotAPI,
	chatID int64,
	cfg *config.AppConfig,
	st *store.Store,
	alerter *alert.Alerter,
	prov provider.Provider,
) *CommandHandler {
	return &CommandHandler{
		bot:     bot,
		chatID:  chatID,
		cfg:     cfg,
		store:   st,
		alerter: alerter,
		prov:    prov,
	}
}

func (h *CommandHandler) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil {
				continue
			}
			// Security: only respond to our own chat
			if update.Message.Chat.ID != h.chatID {
				log.Printf("Ignoring message from unauthorized chat %d", update.Message.Chat.ID)
				continue
			}
			h.handleUpdate(update)
		}
	}
}

func (h *CommandHandler) handleUpdate(update tgbotapi.Update) {
	msg := update.Message
	if !msg.IsCommand() {
		return
	}

	command := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())

	var reply string
	switch command {
	case "status":
		reply = h.cmdStatus()
	case "portfolio":
		reply = h.cmdPortfolio()
	case "bought":
		reply = h.cmdBought(args)
	case "adjust":
		reply = h.cmdAdjust(args)
	case "pause":
		reply = h.cmdPause()
	case "resume":
		reply = h.cmdResume()
	case "help", "start":
		reply = h.cmdHelp()
	default:
		reply = fmt.Sprintf("Lệnh /%s không được hỗ trợ. Dùng /help để xem danh sách.", command)
	}

	if reply != "" {
		if err := h.alerter.SendTo(h.chatID, reply); err != nil {
			log.Printf("Failed to send command reply: %v", err)
		}
	}
}

func (h *CommandHandler) cmdStatus() string {
	stocks := h.cfg.GetStocks()
	quotes := make([]provider.Quote, 0, len(stocks))
	for _, sc := range stocks {
		q, err := h.prov.GetPrice(sc.Code)
		if err != nil {
			log.Printf("cmdStatus: get price %s: %v", sc.Code, err)
			q = provider.Quote{Code: sc.Code, Price: 0}
		}
		quotes = append(quotes, q)
	}

	return alert.FormatStatus(quotes, h.cfg)
}

func (h *CommandHandler) cmdPortfolio() string {
	stocks := h.cfg.GetStocks()
	quotes := make([]provider.Quote, 0, len(stocks))
	for _, sc := range stocks {
		q, err := h.prov.GetPrice(sc.Code)
		if err != nil {
			q = provider.Quote{Code: sc.Code}
		}
		quotes = append(quotes, q)
	}

	states := make(map[string]*store.DCAState)
	for _, sc := range stocks {
		state, err := h.store.GetDCAState(sc.Code)
		if err != nil {
			log.Printf("cmdPortfolio: get DCA state %s: %v", sc.Code, err)
		}
		states[sc.Code] = state
	}

	return alert.FormatPortfolio(states, quotes, h.cfg)
}

// cmdBought handles "/bought VNM 57200 60"
func (h *CommandHandler) cmdBought(args string) string {
	parts := strings.Fields(args)
	if len(parts) != 3 {
		return "❌ Cú pháp: /bought &lt;MÃ&gt; &lt;giá&gt; &lt;số_cổ&gt;\nVí dụ: /bought VNM 57200 60"
	}

	code := strings.ToUpper(parts[0])
	price, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || price <= 0 {
		return "❌ Giá không hợp lệ"
	}
	shares, err := strconv.Atoi(parts[2])
	if err != nil || shares <= 0 {
		return "❌ Số cổ không hợp lệ"
	}

	if _, ok := h.cfg.GetStock(code); !ok {
		return fmt.Sprintf("❌ Mã %s không có trong watchlist", code)
	}

	state, err := h.store.GetDCAState(code)
	if err != nil {
		return fmt.Sprintf("❌ Lỗi đọc trạng thái: %v", err)
	}

	strategy := h.cfg.Strategy
	totalRounds := strategy.DCA.Rounds

	if state == nil {
		state = &store.DCAState{
			Code:        code,
			TotalRounds: totalRounds,
		}
	}

	// Update average price
	oldTotal := state.AvgPrice * float64(state.TotalShares)
	newTotal := price * float64(shares)
	newTotalShares := state.TotalShares + shares
	newAvg := (oldTotal + newTotal) / float64(newTotalShares)

	state.AvgPrice = newAvg
	state.TotalShares = newTotalShares
	state.TotalInvested += price * float64(shares)
	state.RoundsBought++
	state.LastBuyDate = time.Now()

	if err := h.store.SaveDCAState(*state); err != nil {
		return fmt.Sprintf("❌ Lỗi lưu trạng thái: %v", err)
	}

	return fmt.Sprintf(
		"✅ Đã ghi nhận mua <b>%s</b>\n%d cổ @ %s VND\nGiá vốn TB: %s VND\nTổng: %d cổ | DCA: %d/%d đợt",
		code,
		shares,
		alert.FormatVND(price),
		alert.FormatVND(newAvg),
		newTotalShares,
		state.RoundsBought,
		state.TotalRounds,
	)
}

// cmdAdjust handles "/adjust VNM buy_good 55000 57000"
func (h *CommandHandler) cmdAdjust(args string) string {
	parts := strings.Fields(args)
	if len(parts) != 4 {
		return "❌ Cú pháp: /adjust &lt;MÃ&gt; &lt;buy_good|buy_great&gt; &lt;low&gt; &lt;high&gt;\nVí dụ: /adjust VNM buy_good 55000 57000"
	}

	code := strings.ToUpper(parts[0])
	zone := strings.ToLower(parts[1])
	low, err1 := strconv.ParseFloat(parts[2], 64)
	high, err2 := strconv.ParseFloat(parts[3], 64)
	if err1 != nil || err2 != nil || low <= 0 || high <= 0 || low >= high {
		return "❌ Giá không hợp lệ (low phải nhỏ hơn high)"
	}

	if err := h.cfg.UpdateBuyZone(code, zone, low, high); err != nil {
		return fmt.Sprintf("❌ %v", err)
	}

	// Persist to config_cache
	type zoneOverride struct {
		Code string  `json:"code"`
		Zone string  `json:"zone"`
		Low  float64 `json:"low"`
		High float64 `json:"high"`
	}
	key := fmt.Sprintf("%s:%s", code, zone)
	data, _ := json.Marshal(zoneOverride{Code: code, Zone: zone, Low: low, High: high})
	if err := h.store.SaveConfigOverride(key, data); err != nil {
		log.Printf("cmdAdjust: save config override: %v", err)
	}

	return fmt.Sprintf("✅ Đã cập nhật vùng %s của <b>%s</b>\nMới: %s - %s VND",
		zone, code, alert.FormatVND(low), alert.FormatVND(high))
}

func (h *CommandHandler) cmdPause() string {
	h.alerter.Pause()
	return "⏸ Đã tạm dừng tất cả alert. Dùng /resume để bật lại."
}

func (h *CommandHandler) cmdResume() string {
	h.alerter.Resume()
	return "▶️ Đã bật lại alert."
}

func (h *CommandHandler) cmdHelp() string {
	return `📖 <b>DANH SÁCH LỆNH</b>

/status — Giá hiện tại + trạng thái thị trường
/portfolio — Danh mục đã mua + P&L
/bought &lt;MÃ&gt; &lt;giá&gt; &lt;cổ&gt; — Ghi nhận đã mua
/adjust &lt;MÃ&gt; &lt;zone&gt; &lt;low&gt; &lt;high&gt; — Điều chỉnh vùng giá
/pause — Tạm dừng alert
/resume — Bật lại alert
/help — Danh sách lệnh này

<i>Ví dụ:</i>
<code>/bought VNM 57200 60</code>
<code>/adjust ACB buy_good 21000 22000</code>`
}

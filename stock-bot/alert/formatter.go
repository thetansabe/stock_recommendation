package alert

import (
	"fmt"
	"html"
	"strings"
	"time"

	"stock-bot/config"
	"stock-bot/engine"
	"stock-bot/provider"
	"stock-bot/store"
)

// commaSep formats an integer with thousand separators (e.g. 1234567 → "1,234,567").
func commaSep(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func fvnd(v float64) string {
	return commaSep(int64(v))
}

// FormatVND is exported for use by other packages (e.g. cmd).
func FormatVND(v float64) string { return fvnd(v) }

func fpct(v float64) string {
	sign := "+"
	if v < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.2f%%", sign, v)
}

func esc(s string) string {
	return html.EscapeString(s)
}

const divider = "────────────────────────"

func FormatBuySignal(
	stockCfg config.StockConfig,
	sig engine.Signal,
	price float64,
	zone config.PriceZone,
	rec engine.BuyRecommendation,
) string {
	var emoji, label string
	if sig == engine.SignalBuyGreat {
		emoji = "🟢"
		label = "VÀO VÙNG MUA TUYỆT VỜI"
	} else {
		emoji = "🟡"
		label = "VÀO VÙNG MUA TỐT"
	}

	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf("%s <b>%s %s</b>\n", emoji, esc(stockCfg.Code), esc(label)))
	sb.WriteString(fmt.Sprintf("Giá: %s VND\n", fvnd(price)))
	sb.WriteString(fmt.Sprintf("Vùng: %s - %s\n", fvnd(zone.Low), fvnd(zone.High)))
	if rec.ShouldBuy {
		sb.WriteString(fmt.Sprintf("Đợt: %d/%d — Mua %s VND (~%d cổ)\n",
			rec.Round, rec.TotalRounds, fvnd(rec.Amount), rec.SharesToBuy))
	} else {
		sb.WriteString(fmt.Sprintf("<i>%s</i>\n", esc(rec.Reason)))
	}
	sb.WriteString(divider)
	return sb.String()
}

func FormatStopLoss(stockCfg config.StockConfig, price float64, state *store.DCAState) string {
	lossEst := ""
	if state != nil && state.TotalShares > 0 {
		loss := (price - state.AvgPrice) * float64(state.TotalShares)
		lossEst = fmt.Sprintf("\nLỗ ước tính: %s VND", fvnd(loss))
	}

	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf("🔴 <b>%s XUYÊN STOP LOSS</b>\n", esc(stockCfg.Code)))
	sb.WriteString(fmt.Sprintf("Giá: %s VND\n", fvnd(price)))
	sb.WriteString(fmt.Sprintf("Stop loss: %s VND", fvnd(stockCfg.StopLoss)))
	sb.WriteString(lossEst + "\n")
	sb.WriteString("⚡ Cân nhắc bán ngay phiên tới\n")
	sb.WriteString(divider)
	return sb.String()
}

func FormatTP(stockCfg config.StockConfig, sig engine.Signal, price float64, state *store.DCAState) string {
	var tpLabel string
	var tpPrice float64
	if sig == engine.SignalTP2 {
		tpLabel = "TP2"
		tpPrice = stockCfg.TP2
	} else {
		tpLabel = "TP1"
		tpPrice = stockCfg.TP1
	}

	profitStr := ""
	if state != nil && state.TotalShares > 0 && state.AvgPrice > 0 {
		pctGain := (price - state.AvgPrice) / state.AvgPrice * 100
		profit := (price - state.AvgPrice) * float64(state.TotalShares)
		profitStr = fmt.Sprintf("\n+%.1f%% từ entry | Lãi: +%s VND", pctGain, fvnd(profit))
	}

	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf("🎯 <b>%s CHẠM %s</b>\n", esc(stockCfg.Code), tpLabel))
	sb.WriteString(fmt.Sprintf("Giá: %s VND\n", fvnd(price)))
	sb.WriteString(fmt.Sprintf("%s: %s VND", tpLabel, fvnd(tpPrice)))
	sb.WriteString(profitStr + "\n")
	if sig == engine.SignalTP1 {
		sb.WriteString("👉 Cân nhắc bán 50% vị thế\n")
	} else {
		sb.WriteString("👉 Cân nhắc chốt toàn bộ vị thế\n")
	}
	sb.WriteString(divider)
	return sb.String()
}

func FormatDailyReport(
	quotes []provider.Quote,
	states map[string]*store.DCAState,
	cfg *config.AppConfig,
	totalCapital float64,
) string {
	now := time.Now()
	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf("📊 <b>BÁO CÁO NGÀY %s</b>\n\n", now.Format("02/01/2006")))

	for _, q := range quotes {
		sc, _ := cfg.GetStock(q.Code)
		status := watchStatus(q.Price, sc)
		sb.WriteString(fmt.Sprintf("%-4s %s (%s)  %s\n",
			q.Code, fvnd(q.Price), fpct(q.PctChange), status))
	}

	sb.WriteString("\nTrạng thái DCA:\n")
	for _, q := range quotes {
		state := states[q.Code]
		bought := 0
		total := cfg.Strategy.DCA.Rounds
		if state != nil {
			bought = state.RoundsBought
		}
		sb.WriteString(fmt.Sprintf("%s: %d/%d đợt\n", q.Code, bought, total))
	}

	invested := 0.0
	for _, state := range states {
		if state != nil {
			invested += state.TotalInvested
		}
	}
	remaining := totalCapital - invested
	sb.WriteString(fmt.Sprintf("\n💰 Tiền mặt còn lại: %s VND\n", fvnd(remaining)))
	sb.WriteString(divider)
	return sb.String()
}

func watchStatus(price float64, sc config.StockConfig) string {
	if price <= sc.StopLoss {
		return "🔴 Stop loss"
	}
	if price >= sc.BuyGreat.Low && price <= sc.BuyGreat.High {
		return "🟢 Vùng mua tuyệt vời"
	}
	if price >= sc.BuyGood.Low && price <= sc.BuyGood.High {
		return "🟡 Vùng mua tốt"
	}
	if price >= sc.TP2 {
		return "🎯 Chạm TP2"
	}
	if price >= sc.TP1 {
		return "🎯 Chạm TP1"
	}
	if price < sc.BuyGood.High {
		dist := sc.BuyGood.High - price
		return fmt.Sprintf("⏳ Gần vùng mua (-%.0f)", dist)
	}
	return "⏳ Chờ"
}

func FormatStatus(quotes []provider.Quote, cfg *config.AppConfig) string {
	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString("📈 <b>TRẠNG THÁI THỊ TRƯỜNG</b>\n\n")

	for _, q := range quotes {
		sc, _ := cfg.GetStock(q.Code)
		status := watchStatus(q.Price, sc)
		distGood := sc.BuyGood.Low - q.Price
		sb.WriteString(fmt.Sprintf("<b>%s</b> %s VND (%s)\n", q.Code, fvnd(q.Price), fpct(q.PctChange)))
		sb.WriteString(fmt.Sprintf("  %s", status))
		if distGood > 0 {
			sb.WriteString(fmt.Sprintf(" | Cách vùng mua: %.0f VND", distGood))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(divider)
	return sb.String()
}

func FormatPortfolio(states map[string]*store.DCAState, quotes []provider.Quote, cfg *config.AppConfig) string {
	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString("💼 <b>DANH MỤC ĐẦU TƯ</b>\n\n")

	totalPL := 0.0
	totalInvested := 0.0
	quoteMap := make(map[string]float64)
	for _, q := range quotes {
		quoteMap[q.Code] = q.Price
	}

	for _, q := range quotes {
		state := states[q.Code]
		if state == nil || state.TotalShares == 0 {
			sb.WriteString(fmt.Sprintf("<b>%s</b>: Chưa mua\n", q.Code))
			continue
		}
		currentPrice := quoteMap[q.Code]
		pl := (currentPrice - state.AvgPrice) * float64(state.TotalShares)
		plPct := (currentPrice - state.AvgPrice) / state.AvgPrice * 100
		totalPL += pl
		totalInvested += state.TotalInvested

		plSign := "+"
		if pl < 0 {
			plSign = ""
		}

		sb.WriteString(fmt.Sprintf("<b>%s</b> — %d cổ\n", q.Code, state.TotalShares))
		sb.WriteString(fmt.Sprintf("  Giá vốn: %s | Hiện tại: %s\n", fvnd(state.AvgPrice), fvnd(currentPrice)))
		sb.WriteString(fmt.Sprintf("  P&L: %s%s VND (%.1f%%)\n", plSign, fvnd(pl), plPct))
		sb.WriteString(fmt.Sprintf("  DCA: %d/%d đợt\n", state.RoundsBought, state.TotalRounds))
	}

	if totalInvested > 0 {
		plPct := totalPL / totalInvested * 100
		sb.WriteString(fmt.Sprintf("\nTổng P&L: %s VND (%.1f%%)\n", fvnd(totalPL), plPct))
	}
	sb.WriteString(divider)
	return sb.String()
}

func FormatWeekendSummary(quotes []provider.Quote) string {
	now := time.Now()
	var sb strings.Builder
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf("📅 <b>TÓM TẮT TUẦN — %s</b>\n\n", now.Format("02/01/2006")))

	for _, q := range quotes {
		sb.WriteString(fmt.Sprintf("%-4s %s VND (%s)\n", q.Code, fvnd(q.Price), fpct(q.PctChange)))
	}

	sb.WriteString(divider)
	return sb.String()
}

func FormatProviderError() string {
	return "⚠️ <b>Không lấy được giá</b>\nCả SSI và CafeF đều lỗi. Kiểm tra kết nối mạng."
}

package engine

import (
	"fmt"

	"stock-bot/config"
	"stock-bot/store"
)

// CheckPrice compares current price against configured zones and returns the
// highest-priority signal. Priority: StopLoss > BuyGreat > BuyGood > TP2 > TP1 > Watch.
func CheckPrice(price float64, cfg config.StockConfig, state *store.DCAState) SignalResult {
	base := SignalResult{
		Code:  cfg.Code,
		Price: price,
	}

	// 1. Stop loss — highest priority
	if price <= cfg.StopLoss {
		base.Signal = SignalStopLoss
		base.Ref = cfg.StopLoss
		base.Message = fmt.Sprintf("Giá %.0f xuyên stop loss %.0f", price, cfg.StopLoss)
		return base
	}

	// 2. Buy great
	if price >= cfg.BuyGreat.Low && price <= cfg.BuyGreat.High {
		base.Signal = SignalBuyGreat
		base.Ref = cfg.BuyGreat.High
		base.Message = fmt.Sprintf("Giá %.0f vào vùng mua tuyệt vời [%.0f - %.0f]", price, cfg.BuyGreat.Low, cfg.BuyGreat.High)
		return base
	}

	// 3. Buy good
	if price >= cfg.BuyGood.Low && price <= cfg.BuyGood.High {
		base.Signal = SignalBuyGood
		base.Ref = cfg.BuyGood.High
		base.Message = fmt.Sprintf("Giá %.0f vào vùng mua tốt [%.0f - %.0f]", price, cfg.BuyGood.Low, cfg.BuyGood.High)
		return base
	}

	// 4. TP2 (check before TP1 since TP2 > TP1)
	if price >= cfg.TP2 {
		if state == nil || !state.TP2Sold {
			base.Signal = SignalTP2
			base.Ref = cfg.TP2
			base.Message = fmt.Sprintf("Giá %.0f chạm TP2 %.0f", price, cfg.TP2)
			return base
		}
	}

	// 5. TP1
	if price >= cfg.TP1 {
		if state == nil || !state.TP1Sold {
			base.Signal = SignalTP1
			base.Ref = cfg.TP1
			base.Message = fmt.Sprintf("Giá %.0f chạm TP1 %.0f", price, cfg.TP1)
			return base
		}
	}

	// 6. Watch
	base.Signal = SignalWatch
	base.Message = "Theo dõi, chưa vào vùng"
	return base
}

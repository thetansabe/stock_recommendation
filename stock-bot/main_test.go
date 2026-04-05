package main

import (
	"fmt"
	"testing"
	"time"

	"stock-bot/config"
	"stock-bot/engine"
	"stock-bot/provider"
)

func TestConsoleCheck(t *testing.T) {
	cfg, err := config.Load("config/watchlist.yaml", "config/strategy.yaml")
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}

	prov := provider.NewFallback(
		provider.WithRetry(provider.NewSSI(), 2, 3*time.Second),
		provider.WithRetry(provider.NewCafeF(), 2, 3*time.Second),
	)

	fmt.Println("────────────────────────────────────────")

	for _, sc := range cfg.GetStocks() {
		quote, err := prov.GetPrice(sc.Code)
		if err != nil {
			t.Logf("⚠️  %s: lỗi lấy giá (%v)", sc.Code, err)
			continue
		}

		result := engine.CheckPrice(quote.Price, sc, nil)
		rec := engine.EvaluateBuy(sc.Code, quote.Price, cfg, nil)

		fmt.Printf("\n📊 %s (%s)\n", sc.Code, sc.Name)
		fmt.Printf("   Giá:        %.0f VND (%+.2f%%)\n", quote.Price, quote.PctChange)
		fmt.Printf("   Volume:     %d\n", quote.Volume)
		fmt.Printf("   Signal:     %s — %s\n", result.Signal, result.Message)
		fmt.Printf("   Vùng tốt:   %.0f - %.0f\n", sc.BuyGood.Low, sc.BuyGood.High)
		fmt.Printf("   Vùng tuyệt: %.0f - %.0f\n", sc.BuyGreat.Low, sc.BuyGreat.High)
		fmt.Printf("   Stop loss:  %.0f | TP1: %.0f | TP2: %.0f\n", sc.StopLoss, sc.TP1, sc.TP2)
		if rec.ShouldBuy {
			fmt.Printf("   ✅ %s\n", rec.Reason)
		} else {
			fmt.Printf("   ⏳ %s\n", rec.Reason)
		}
	}

	fmt.Println("\n────────────────────────────────────────")
}

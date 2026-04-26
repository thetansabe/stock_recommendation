package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"stock-bot/alert"
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

func TestSendToTelegram(t *testing.T) {
	_ = godotenv.Load()
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatIDStr == "" {
		t.Skip("TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID not set")
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		t.Fatalf("invalid TELEGRAM_CHAT_ID: %v", err)
	}

	cfg, err := config.Load("config/watchlist.yaml", "config/strategy.yaml")
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}

	prov := provider.NewFallback(
		provider.WithRetry(provider.NewSSI(), 2, 3*time.Second),
		provider.WithRetry(provider.NewCafeF(), 2, 3*time.Second),
	)

	var quotes []provider.Quote
	for _, sc := range cfg.GetStocks() {
		q, err := prov.GetPrice(sc.Code)
		if err != nil {
			t.Logf("get price %s: %v", sc.Code, err)
			q = provider.Quote{Code: sc.Code}
		}
		quotes = append(quotes, q)
	}

	alerter, err := alert.NewAlerter(token, chatID)
	if err != nil {
		t.Fatalf("init alerter: %v", err)
	}

	msg := alert.FormatStatus(quotes, cfg)
	if err := alerter.Send(msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Log("sent to Telegram OK")
}

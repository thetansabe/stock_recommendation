package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/joho/godotenv"

	"stock-bot/alert"
	"stock-bot/cmd"
	"stock-bot/config"
	"stock-bot/engine"
	"stock-bot/provider"
	"stock-bot/store"
)

func main() {
	// Load .env (ignore error — vars may already be set in environment)
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chatIDStr == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must be set")
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID: %v", err)
	}

	// Load config
	cfg, err := config.Load("config/watchlist.yaml", "config/strategy.yaml")
	if err != nil {
		log.Fatalf("Load config: %v", err)
	}

	// Open BoltDB
	st, err := store.Open("stock-bot.db")
	if err != nil {
		log.Fatalf("Open store: %v", err)
	}
	defer st.Close()

	// Restore /adjust overrides from config_cache
	applyConfigOverrides(cfg, st)

	// Build provider chain: SSI (retry 3x) → CafeF (retry 3x)
	ssiProv := provider.WithRetry(provider.NewSSI(), 3, 5*time.Second)
	cafefProv := provider.WithRetry(provider.NewCafeF(), 3, 5*time.Second)
	prov := provider.NewFallback(ssiProv, cafefProv)

	// Init alerter
	alerter, err := alert.NewAlerter(token, chatID)
	if err != nil {
		log.Fatalf("Init alerter: %v", err)
	}

	// Init command handler
	handler := cmd.NewCommandHandler(alerter.Bot(), chatID, cfg, st, alerter, prov)

	// Init scheduler with Vietnam timezone
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("Cannot load Asia/Ho_Chi_Minh timezone, falling back to UTC+7: %v", err)
		loc = time.FixedZone("ICT", 7*60*60)
	}

	s, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		log.Fatalf("Init scheduler: %v", err)
	}
	defer s.Shutdown()

	// Job: check prices every 5 minutes during market hours
	_, err = s.NewJob(
		gocron.CronJob(cfg.Strategy.Schedule.CheckInterval, false),
		gocron.NewTask(checkPriceJob, cfg, st, prov, alerter),
		gocron.WithName("check_price"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		log.Fatalf("Register check_price job: %v", err)
	}

	// Job: daily report at 8 PM
	_, err = s.NewJob(
		gocron.CronJob(cfg.Strategy.Schedule.DailyReport, false),
		gocron.NewTask(dailyReportJob, cfg, st, prov, alerter),
		gocron.WithName("daily_report"),
	)
	if err != nil {
		log.Fatalf("Register daily_report job: %v", err)
	}

	// Job: weekend summary at 10 AM Saturday
	_, err = s.NewJob(
		gocron.CronJob(cfg.Strategy.Schedule.WeekendSummary, false),
		gocron.NewTask(weekendSummaryJob, cfg, st, prov, alerter),
		gocron.WithName("weekend_summary"),
	)
	if err != nil {
		log.Fatalf("Register weekend_summary job: %v", err)
	}

	// Start scheduler (non-blocking)
	s.Start()
	log.Println("Scheduler started")

	// Start Telegram command polling goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handler.Start(ctx)
	log.Printf("Bot started. Monitoring: VNM, ACB, HPG")

	// Block until OS signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")
}

// checkPriceJob runs every 5 minutes during market hours.
func checkPriceJob(cfg *config.AppConfig, st *store.Store, prov provider.Provider, alerter *alert.Alerter) {
	for _, stockCfg := range cfg.GetStocks() {
		quote, err := prov.GetPrice(stockCfg.Code)
		if err != nil {
			log.Printf("checkPriceJob: get price %s: %v", stockCfg.Code, err)
			continue
		}

		dcaState, err := st.GetDCAState(stockCfg.Code)
		if err != nil {
			log.Printf("checkPriceJob: get DCA state %s: %v", stockCfg.Code, err)
		}

		result := engine.CheckPrice(quote.Price, stockCfg, dcaState)

		// Log price regardless of signal
		_ = st.LogPrice(store.PriceEntry{
			Code:      stockCfg.Code,
			Price:     quote.Price,
			Change:    quote.Change,
			PctChange: quote.PctChange,
			Volume:    quote.Volume,
			Timestamp: time.Now(),
		})

		if result.Signal == engine.SignalWatch || result.Signal == engine.SignalNone {
			continue
		}

		// Anti-spam: skip if same signal sent within 2 hours (except STOP_LOSS)
		if result.Signal != engine.SignalStopLoss {
			lastTime, exists, _ := st.GetLastSignalTime(stockCfg.Code, string(result.Signal))
			if exists && time.Since(lastTime) < 2*time.Hour {
				continue
			}
		}

		var msg string
		switch result.Signal {
		case engine.SignalBuyGood, engine.SignalBuyGreat:
			var zone config.PriceZone
			if result.Signal == engine.SignalBuyGreat {
				zone = stockCfg.BuyGreat
			} else {
				zone = stockCfg.BuyGood
			}
			rec := engine.EvaluateBuy(stockCfg.Code, quote.Price, cfg, dcaState)
			msg = alert.FormatBuySignal(stockCfg, result.Signal, quote.Price, zone, rec)
		case engine.SignalStopLoss:
			msg = alert.FormatStopLoss(stockCfg, quote.Price, dcaState)
		case engine.SignalTP1, engine.SignalTP2:
			msg = alert.FormatTP(stockCfg, result.Signal, quote.Price, dcaState)
		}

		if msg != "" {
			if err := alerter.Send(msg); err != nil {
				log.Printf("checkPriceJob: send alert %s %s: %v", stockCfg.Code, result.Signal, err)
			} else {
				_ = st.LogSignal(stockCfg.Code, string(result.Signal))
			}
		}
	}
}

// dailyReportJob sends end-of-day summary at 8 PM.
func dailyReportJob(cfg *config.AppConfig, st *store.Store, prov provider.Provider, alerter *alert.Alerter) {
	stocks := cfg.GetStocks()
	quotes := make([]provider.Quote, 0, len(stocks))
	for _, sc := range stocks {
		q, err := prov.GetPrice(sc.Code)
		if err != nil {
			log.Printf("dailyReportJob: get price %s: %v", sc.Code, err)
			q = provider.Quote{Code: sc.Code}
		}
		quotes = append(quotes, q)
	}

	states := make(map[string]*store.DCAState)
	for _, sc := range stocks {
		state, _ := st.GetDCAState(sc.Code)
		states[sc.Code] = state
	}

	msg := alert.FormatDailyReport(quotes, states, cfg, cfg.Strategy.Portfolio.TotalCapital)
	if err := alerter.Send(msg); err != nil {
		log.Printf("dailyReportJob: send: %v", err)
	}

	// Prune old price logs (keep 30 days)
	if err := st.PruneOldPriceLogs(30 * 24 * time.Hour); err != nil {
		log.Printf("dailyReportJob: prune: %v", err)
	}
}

// weekendSummaryJob sends a weekly summary on Saturday at 10 AM.
func weekendSummaryJob(cfg *config.AppConfig, st *store.Store, prov provider.Provider, alerter *alert.Alerter) {
	stocks := cfg.GetStocks()
	quotes := make([]provider.Quote, 0, len(stocks))
	for _, sc := range stocks {
		q, err := prov.GetPrice(sc.Code)
		if err != nil {
			q = provider.Quote{Code: sc.Code}
		}
		quotes = append(quotes, q)
	}

	msg := alert.FormatWeekendSummary(quotes)
	if err := alerter.Send(msg); err != nil {
		log.Printf("weekendSummaryJob: send: %v", err)
	}
}

// applyConfigOverrides restores /adjust overrides from the config_cache bucket.
type zoneOverride struct {
	Code string  `json:"code"`
	Zone string  `json:"zone"`
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

func applyConfigOverrides(cfg *config.AppConfig, st *store.Store) {
	err := st.IterConfigOverrides(func(key string, data []byte) error {
		var ov zoneOverride
		if err := json.Unmarshal(data, &ov); err != nil {
			log.Printf("applyConfigOverrides: unmarshal key %q: %v", key, err)
			return nil
		}
		if err := cfg.UpdateBuyZone(ov.Code, ov.Zone, ov.Low, ov.High); err != nil {
			log.Printf("applyConfigOverrides: apply %q: %v", key, err)
		}
		return nil
	})
	if err != nil {
		log.Printf("applyConfigOverrides: iterate: %v", err)
	}
}

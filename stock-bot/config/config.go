package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type PriceZone struct {
	Low  float64 `yaml:"low"`
	High float64 `yaml:"high"`
}

type StockConfig struct {
	Code     string    `yaml:"code"`
	Name     string    `yaml:"name"`
	Exchange string    `yaml:"exchange"`
	BuyGood  PriceZone `yaml:"buy_good"`
	BuyGreat PriceZone `yaml:"buy_great"`
	StopLoss float64   `yaml:"stop_loss"`
	TP1      float64   `yaml:"tp1"`
	TP2      float64   `yaml:"tp2"`
}

type Watchlist struct {
	Stocks []StockConfig `yaml:"stocks"`
}

type AllocationMap map[string]float64

type DCAConfig struct {
	Rounds       int `yaml:"rounds"`
	IntervalDays int `yaml:"interval_days"`
}

type MarketConditions struct {
	VNIndexFloor    float64 `yaml:"vnindex_floor"`
	OilPriceCeiling float64 `yaml:"oil_price_ceiling"`
}

type ScheduleConfig struct {
	CheckInterval  string `yaml:"check_interval"`
	DailyReport    string `yaml:"daily_report"`
	WeekendSummary string `yaml:"weekend_summary"`
}

type Portfolio struct {
	TotalCapital float64       `yaml:"total_capital"`
	Allocation   AllocationMap `yaml:"allocation"`
}

type Strategy struct {
	Portfolio        Portfolio        `yaml:"portfolio"`
	DCA              DCAConfig        `yaml:"dca"`
	MarketConditions MarketConditions `yaml:"market_conditions"`
	Schedule         ScheduleConfig   `yaml:"schedule"`
}

type AppConfig struct {
	mu        sync.RWMutex
	Watchlist Watchlist
	Strategy  Strategy
}

func Load(watchlistPath, strategyPath string) (*AppConfig, error) {
	cfg := &AppConfig{}

	wData, err := os.ReadFile(watchlistPath)
	if err != nil {
		return nil, fmt.Errorf("read watchlist: %w", err)
	}
	if err := yaml.Unmarshal(wData, &cfg.Watchlist); err != nil {
		return nil, fmt.Errorf("parse watchlist: %w", err)
	}

	sData, err := os.ReadFile(strategyPath)
	if err != nil {
		return nil, fmt.Errorf("read strategy: %w", err)
	}
	if err := yaml.Unmarshal(sData, &cfg.Strategy); err != nil {
		return nil, fmt.Errorf("parse strategy: %w", err)
	}

	return cfg, nil
}

func (c *AppConfig) GetStock(code string) (StockConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, s := range c.Watchlist.Stocks {
		if s.Code == code {
			return s, true
		}
	}
	return StockConfig{}, false
}

func (c *AppConfig) GetStocks() []StockConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]StockConfig, len(c.Watchlist.Stocks))
	copy(result, c.Watchlist.Stocks)
	return result
}

func (c *AppConfig) UpdateBuyZone(code, zone string, low, high float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, s := range c.Watchlist.Stocks {
		if s.Code == code {
			switch zone {
			case "buy_good":
				c.Watchlist.Stocks[i].BuyGood = PriceZone{Low: low, High: high}
			case "buy_great":
				c.Watchlist.Stocks[i].BuyGreat = PriceZone{Low: low, High: high}
			default:
				return fmt.Errorf("unknown zone %q: must be buy_good or buy_great", zone)
			}
			return nil
		}
	}
	return fmt.Errorf("stock %q not found", code)
}

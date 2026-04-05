package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const cafefURL = "https://banggia.cafef.vn/stockhandler.ashx"

type CafeFProvider struct {
	client  *http.Client
	cache   []cafefStock
	cacheAt time.Time
}

func NewCafeF() *CafeFProvider {
	return &CafeFProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *CafeFProvider) Name() string { return "CafeF" }

// cafefStock maps fields from banggia.cafef.vn JSON.
// Prices are in thousands (e.g. 6.94 = 6,940 VND).
type cafefStock struct {
	A           string  `json:"a"`           // symbol
	B           float64 `json:"b"`           // ref price (thousands)
	K           float64 `json:"k"`           // price change (thousands)
	L           float64 `json:"l"`           // last/match price (thousands)
	N           float64 `json:"n"`           // total volume
	TotalVolume float64 `json:"totalvolume"` // total volume (alternative)
}

func (p *CafeFProvider) fetchAll() ([]cafefStock, error) {
	if time.Since(p.cacheAt) < 30*time.Second && len(p.cache) > 0 {
		return p.cache, nil
	}

	req, err := http.NewRequest("GET", cafefURL+"?index=HOSE", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://banggia.cafef.vn/")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var stocks []cafefStock
	if err := json.NewDecoder(resp.Body).Decode(&stocks); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	p.cache = stocks
	p.cacheAt = time.Now()
	return p.cache, nil
}

func (p *CafeFProvider) GetPrice(code string) (Quote, error) {
	stocks, err := p.fetchAll()
	if err != nil {
		return Quote{}, err
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	for _, s := range stocks {
		if strings.ToUpper(strings.TrimSpace(s.A)) == code {
			price := s.L * 1000 // convert from thousands to VND
			if price == 0 {
				return Quote{}, fmt.Errorf("zero price for %s", code)
			}
			ref := s.B * 1000
			change := s.K * 1000
			var pctChange float64
			if ref > 0 {
				pctChange = change / ref * 100
			}
			vol := int64(s.N)
			if vol == 0 {
				vol = int64(s.TotalVolume)
			}
			return Quote{
				Code:      code,
				Price:     price,
				Change:    change,
				PctChange: pctChange,
				Volume:    vol,
			}, nil
		}
	}
	return Quote{}, fmt.Errorf("stock %s not found in CafeF response", code)
}

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const ssiURL = "https://iboard-query.ssi.com.vn/stock/type/s/hose"

type SSIProvider struct {
	client  *http.Client
	cache   []ssiStock
	cacheAt time.Time
}

func NewSSI() *SSIProvider {
	return &SSIProvider{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *SSIProvider) Name() string { return "SSI" }

type ssiResponse struct {
	Code    string     `json:"code"`
	Message string     `json:"message"`
	Data    []ssiStock `json:"data"`
}

type ssiStock struct {
	StockSymbol        string  `json:"stockSymbol"`
	RefPrice           float64 `json:"refPrice"`
	MatchedPrice       float64 `json:"matchedPrice"`
	PriceChange        float64 `json:"priceChange"`
	PriceChangePercent float64 `json:"priceChangePercent"`
	NmTotalTradedQty   float64 `json:"nmTotalTradedQty"`
}

func (p *SSIProvider) fetchAll() ([]ssiStock, error) {
	if time.Since(p.cacheAt) < 30*time.Second && len(p.cache) > 0 {
		return p.cache, nil
	}

	req, err := http.NewRequest("GET", ssiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var result ssiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	if result.Code != "SUCCESS" {
		return nil, fmt.Errorf("API error: %s", result.Message)
	}

	p.cache = result.Data
	p.cacheAt = time.Now()
	return p.cache, nil
}

func (p *SSIProvider) GetPrice(code string) (Quote, error) {
	stocks, err := p.fetchAll()
	if err != nil {
		return Quote{}, err
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	for _, s := range stocks {
		if strings.ToUpper(s.StockSymbol) == code {
			price := s.MatchedPrice
			if price == 0 {
				price = s.RefPrice
			}
			if price == 0 {
				return Quote{}, fmt.Errorf("zero price for %s", code)
			}
			return Quote{
				Code:      code,
				Price:     price,
				Change:    s.PriceChange,
				PctChange: s.PriceChangePercent,
				Volume:    int64(s.NmTotalTradedQty),
			}, nil
		}
	}
	return Quote{}, fmt.Errorf("stock %s not found in SSI response", code)
}

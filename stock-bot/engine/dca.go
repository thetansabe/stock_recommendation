package engine

import (
	"fmt"
	"math"
	"time"

	"stock-bot/config"
	"stock-bot/store"
)

type BuyRecommendation struct {
	ShouldBuy   bool
	Round       int
	TotalRounds int
	Amount      float64 // VND to spend this round
	SharesToBuy int     // rounded down to lot of 100
	Reason      string
}

// EvaluateBuy determines whether a buy should happen for a given stock.
// It enforces DCA rules: round limit, cooldown period.
func EvaluateBuy(
	code string,
	price float64,
	cfg *config.AppConfig,
	state *store.DCAState,
) BuyRecommendation {
	strategy := cfg.Strategy
	totalRounds := strategy.DCA.Rounds
	intervalDays := strategy.DCA.IntervalDays

	allocation, ok := strategy.Portfolio.Allocation[code]
	if !ok {
		return BuyRecommendation{Reason: fmt.Sprintf("Mã %s không có trong allocation", code)}
	}

	totalCapital := strategy.Portfolio.TotalCapital
	perRoundAmount := (totalCapital * allocation) / float64(totalRounds)

	// If no state yet, treat as round 0
	roundsBought := 0
	var lastBuyDate time.Time
	if state != nil {
		roundsBought = state.RoundsBought
		lastBuyDate = state.LastBuyDate
	}

	// Rule 1: Rounds remaining
	if roundsBought >= totalRounds {
		return BuyRecommendation{
			Round:       roundsBought + 1,
			TotalRounds: totalRounds,
			Amount:      perRoundAmount,
			Reason:      fmt.Sprintf("Đã mua đủ %d/%d đợt", roundsBought, totalRounds),
		}
	}

	// Rule 2: Cooldown (skip for first round)
	if roundsBought > 0 && !lastBuyDate.IsZero() {
		daysSince := time.Since(lastBuyDate).Hours() / 24
		if daysSince < float64(intervalDays) {
			return BuyRecommendation{
				Round:       roundsBought + 1,
				TotalRounds: totalRounds,
				Amount:      perRoundAmount,
				Reason:      fmt.Sprintf("Cần chờ thêm %.0f ngày nữa (khoảng cách %d ngày)", float64(intervalDays)-daysSince, intervalDays),
			}
		}
	}

	// Calculate shares (round down to nearest lot of 100)
	rawShares := perRoundAmount / price
	shareLots := int(math.Floor(rawShares/100)) * 100
	if shareLots == 0 {
		shareLots = int(math.Floor(rawShares))
	}

	return BuyRecommendation{
		ShouldBuy:   true,
		Round:       roundsBought + 1,
		TotalRounds: totalRounds,
		Amount:      perRoundAmount,
		SharesToBuy: shareLots,
		Reason:      fmt.Sprintf("Đợt %d/%d — Mua %.0f VND (~%d cổ)", roundsBought+1, totalRounds, perRoundAmount, shareLots),
	}
}

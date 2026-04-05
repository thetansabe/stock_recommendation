package engine

type Signal string

const (
	SignalNone     Signal = "NONE"
	SignalWatch    Signal = "WATCH"
	SignalBuyGood  Signal = "BUY_GOOD"
	SignalBuyGreat Signal = "BUY_GREAT"
	SignalStopLoss Signal = "STOP_LOSS"
	SignalTP1      Signal = "TP1"
	SignalTP2      Signal = "TP2"
)

type SignalResult struct {
	Code    string
	Signal  Signal
	Price   float64
	Ref     float64
	Message string
}

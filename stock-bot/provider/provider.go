package provider

import (
	"log"
	"time"
)

type Quote struct {
	Code      string
	Price     float64
	Change    float64
	PctChange float64
	Volume    int64
}

type Provider interface {
	GetPrice(code string) (Quote, error)
	Name() string
}

// retryProvider wraps a Provider with retry logic.
type retryProvider struct {
	inner       Provider
	maxAttempts int
	delay       time.Duration
}

func WithRetry(p Provider, maxAttempts int, delay time.Duration) Provider {
	return &retryProvider{inner: p, maxAttempts: maxAttempts, delay: delay}
}

func (r *retryProvider) Name() string { return r.inner.Name() }

func (r *retryProvider) GetPrice(code string) (Quote, error) {
	var lastErr error
	for i := 0; i < r.maxAttempts; i++ {
		if i > 0 {
			time.Sleep(r.delay)
		}
		q, err := r.inner.GetPrice(code)
		if err == nil {
			return q, nil
		}
		lastErr = err
		log.Printf("[%s] GetPrice(%s) attempt %d/%d failed: %v", r.inner.Name(), code, i+1, r.maxAttempts, err)
	}
	return Quote{}, lastErr
}

// fallbackProvider tries primary, then secondary on failure.
type fallbackProvider struct {
	primary   Provider
	secondary Provider
}

func NewFallback(primary, secondary Provider) Provider {
	return &fallbackProvider{primary: primary, secondary: secondary}
}

func (f *fallbackProvider) Name() string { return f.primary.Name() + "+fallback" }

func (f *fallbackProvider) GetPrice(code string) (Quote, error) {
	q, err := f.primary.GetPrice(code)
	if err == nil {
		return q, nil
	}
	log.Printf("primary provider failed (%v), trying fallback for %s", err, code)
	return f.secondary.GetPrice(code)
}

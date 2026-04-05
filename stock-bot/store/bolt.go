package store

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketDCAState    = []byte("dca_state")
	bucketPriceLog    = []byte("price_log")
	bucketSignalLog   = []byte("signal_log")
	bucketConfigCache = []byte("config_cache")
)

type DCAState struct {
	Code          string    `json:"code"`
	RoundsBought  int       `json:"rounds_bought"`
	TotalRounds   int       `json:"total_rounds"`
	AvgPrice      float64   `json:"avg_price"`
	TotalShares   int       `json:"total_shares"`
	TotalInvested float64   `json:"total_invested"`
	LastBuyDate   time.Time `json:"last_buy_date"`
	TP1Sold       bool      `json:"tp1_sold"`
	TP2Sold       bool      `json:"tp2_sold"`
}

type PriceEntry struct {
	Code      string    `json:"code"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	PctChange float64   `json:"pct_change"`
	Volume    int64     `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketDCAState, bucketPriceLog, bucketSignalLog, bucketConfigCache} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("init buckets: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetDCAState(code string) (*DCAState, error) {
	var state DCAState
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDCAState)
		raw := b.Get([]byte(code))
		if raw == nil {
			return nil
		}
		cp := make([]byte, len(raw))
		copy(cp, raw)
		return json.Unmarshal(cp, &state)
	})
	if err != nil {
		return nil, err
	}
	if state.Code == "" {
		return nil, nil
	}
	return &state, nil
}

func (s *Store) SaveDCAState(state DCAState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketDCAState).Put([]byte(state.Code), data)
	})
}

func (s *Store) LogPrice(entry PriceEntry) error {
	key := fmt.Sprintf("%s:%s", entry.Code, entry.Timestamp.UTC().Format(time.RFC3339))
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketPriceLog).Put([]byte(key), data)
	})
}

func (s *Store) GetLastSignalTime(code, signal string) (time.Time, bool, error) {
	key := fmt.Sprintf("%s:%s", code, signal)
	var t time.Time
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketSignalLog).Get([]byte(key))
		if raw == nil {
			return nil
		}
		cp := make([]byte, len(raw))
		copy(cp, raw)
		var err error
		t, err = time.Parse(time.RFC3339, string(cp))
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	return t, found, err
}

func (s *Store) LogSignal(code, signal string) error {
	key := fmt.Sprintf("%s:%s", code, signal)
	val := time.Now().UTC().Format(time.RFC3339)
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSignalLog).Put([]byte(key), []byte(val))
	})
}

func (s *Store) PruneOldPriceLogs(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketPriceLog)
		c := b.Cursor()
		var toDelete [][]byte
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			// key format: "VNM:2026-04-05T14:30:00Z"
			// extract timestamp after the first ":"
			key := string(k)
			for i := 0; i < len(key); i++ {
				if key[i] == ':' {
					ts, err := time.Parse(time.RFC3339, key[i+1:])
					if err == nil && ts.Before(cutoff) {
						cp := make([]byte, len(k))
						copy(cp, k)
						toDelete = append(toDelete, cp)
					}
					break
				}
			}
		}
		for _, k := range toDelete {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) SaveConfigOverride(key string, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketConfigCache).Put([]byte(key), data)
	})
}

func (s *Store) GetConfigOverride(key string) ([]byte, bool, error) {
	var result []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketConfigCache).Get([]byte(key))
		if raw == nil {
			return nil
		}
		result = make([]byte, len(raw))
		copy(result, raw)
		return nil
	})
	return result, result != nil, err
}

func (s *Store) IterConfigOverrides(fn func(key string, data []byte) error) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketConfigCache)
		return b.ForEach(func(k, v []byte) error {
			cp := make([]byte, len(v))
			copy(cp, v)
			return fn(string(k), cp)
		})
	})
}

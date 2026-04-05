package alert

import (
	"fmt"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Alerter struct {
	bot    *tgbotapi.BotAPI
	chatID int64
	paused bool
	mu     sync.RWMutex
}

func NewAlerter(token string, chatID int64) (*Alerter, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}
	return &Alerter{bot: bot, chatID: chatID}, nil
}

func (a *Alerter) Bot() *tgbotapi.BotAPI {
	return a.bot
}

func (a *Alerter) Send(text string) error {
	a.mu.RLock()
	paused := a.paused
	a.mu.RUnlock()
	if paused {
		return nil
	}

	msg := tgbotapi.NewMessage(a.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	_, err := a.bot.Send(msg)
	return err
}

func (a *Alerter) SendTo(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	_, err := a.bot.Send(msg)
	return err
}

func (a *Alerter) Pause() {
	a.mu.Lock()
	a.paused = true
	a.mu.Unlock()
}

func (a *Alerter) Resume() {
	a.mu.Lock()
	a.paused = false
	a.mu.Unlock()
}

func (a *Alerter) IsPaused() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.paused
}

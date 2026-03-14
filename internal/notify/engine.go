package notify

import (
	"log/slog"
	"strings"

	"github.com/kaylincoded/magic-guardian/internal/mg"
	"github.com/kaylincoded/magic-guardian/internal/store"
)

// AlertSender is the interface for sending alerts to users.
type AlertSender interface {
	SendStockAlert(userID string, changes []mg.StockChange) error
}

// Engine matches stock changes against subscriptions and dispatches alerts.
type Engine struct {
	store  *store.Store
	sender AlertSender
	logger *slog.Logger
}

// NewEngine creates a new notification engine.
func NewEngine(store *store.Store, sender AlertSender, logger *slog.Logger) *Engine {
	return &Engine{
		store:  store,
		sender: sender,
		logger: logger,
	}
}

// HandleRestocks processes a batch of stock changes from a restock event.
func (e *Engine) HandleRestocks(changes []mg.StockChange) {
	// Group alerts by user: userID → list of changes they care about
	userAlerts := make(map[string][]mg.StockChange)

	for _, ch := range changes {
		itemID := strings.ToLower(ch.Item.ItemID())
		subs, err := e.store.GetSubscribersForItem(itemID)
		if err != nil {
			e.logger.Error("failed to get subscribers", "item", itemID, "error", err)
			continue
		}
		for _, sub := range subs {
			userAlerts[sub.UserID] = append(userAlerts[sub.UserID], ch)
		}
	}

	for userID, alerts := range userAlerts {
		e.logger.Info("sending stock alert", "user", userID, "items", len(alerts))
		if err := e.sender.SendStockAlert(userID, alerts); err != nil {
			e.logger.Error("failed to send alert", "user", userID, "error", err)
		}
	}
}

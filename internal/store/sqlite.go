package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Store manages subscription persistence.
type Store struct {
	db *sql.DB
}

// Subscription represents a user's item subscription.
type Subscription struct {
	ID       int64
	UserID   string
	GuildID  string
	ItemID   string
	ShopType string
}

// New opens or creates a SQLite database at the given path.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// BoardConfig stores the stock board channel and message IDs for a guild.
type BoardConfig struct {
	GuildID  string
	Channels map[string]string // shop_type -> channel_id
	Messages map[string]string // shop_type -> message_id
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS subscriptions (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id   TEXT NOT NULL,
			guild_id  TEXT NOT NULL DEFAULT '',
			item_id   TEXT NOT NULL,
			shop_type TEXT NOT NULL,
			UNIQUE(user_id, item_id)
		);
		CREATE INDEX IF NOT EXISTS idx_subscriptions_item ON subscriptions(item_id);
		CREATE TABLE IF NOT EXISTS board_messages (
			guild_id   TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			shop_type  TEXT NOT NULL,
			message_id TEXT NOT NULL,
			PRIMARY KEY(guild_id, shop_type)
		);
		CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	return err
}

// Subscribe adds a subscription for a user. Returns false if already subscribed.
func (s *Store) Subscribe(userID, guildID, itemID, shopType string) (bool, error) {
	itemID = strings.ToLower(itemID)
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO subscriptions (user_id, guild_id, item_id, shop_type) VALUES (?, ?, ?, ?)`,
		userID, guildID, itemID, shopType,
	)
	if err != nil {
		return false, fmt.Errorf("insert subscription: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// Unsubscribe removes a subscription for a user. Returns false if not found.
func (s *Store) Unsubscribe(userID, itemID string) (bool, error) {
	itemID = strings.ToLower(itemID)
	res, err := s.db.Exec(
		`DELETE FROM subscriptions WHERE user_id = ? AND item_id = ?`,
		userID, itemID,
	)
	if err != nil {
		return false, fmt.Errorf("delete subscription: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

// UnsubscribeAll removes all subscriptions for a user. Returns the count removed.
func (s *Store) UnsubscribeAll(userID string) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM subscriptions WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete all subscriptions: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// GetUserSubscriptions returns all subscriptions for a user.
func (s *Store) GetUserSubscriptions(userID string) ([]Subscription, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, guild_id, item_id, shop_type FROM subscriptions WHERE user_id = ? ORDER BY shop_type, item_id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query subscriptions: %w", err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

// GetSubscribersForItem returns all user IDs subscribed to a given item.
func (s *Store) GetSubscribersForItem(itemID string) ([]Subscription, error) {
	itemID = strings.ToLower(itemID)
	rows, err := s.db.Query(
		`SELECT id, user_id, guild_id, item_id, shop_type FROM subscriptions WHERE item_id = ?`,
		itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("query subscribers: %w", err)
	}
	defer rows.Close()
	return scanSubscriptions(rows)
}

// SetBoardMessage stores a board message ID for a guild/shop.
func (s *Store) SetBoardMessage(guildID, channelID, shopType, messageID string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO board_messages (guild_id, channel_id, shop_type, message_id) VALUES (?, ?, ?, ?)`,
		guildID, channelID, shopType, messageID,
	)
	return err
}

// GetBoardConfig returns the board config for a guild.
func (s *Store) GetBoardConfig(guildID string) (*BoardConfig, error) {
	rows, err := s.db.Query(
		`SELECT channel_id, shop_type, message_id FROM board_messages WHERE guild_id = ?`, guildID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cfg := &BoardConfig{GuildID: guildID, Channels: make(map[string]string), Messages: make(map[string]string)}
	for rows.Next() {
		var channelID, shopType, messageID string
		if err := rows.Scan(&channelID, &shopType, &messageID); err != nil {
			return nil, err
		}
		cfg.Channels[shopType] = channelID
		cfg.Messages[shopType] = messageID
	}
	if len(cfg.Messages) == 0 {
		return nil, nil
	}
	return cfg, rows.Err()
}

// GetAllBoardConfigs returns board configs for all guilds.
func (s *Store) GetAllBoardConfigs() ([]BoardConfig, error) {
	rows, err := s.db.Query(`SELECT guild_id, channel_id, shop_type, message_id FROM board_messages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byGuild := make(map[string]*BoardConfig)
	for rows.Next() {
		var guildID, channelID, shopType, messageID string
		if err := rows.Scan(&guildID, &channelID, &shopType, &messageID); err != nil {
			return nil, err
		}
		if _, ok := byGuild[guildID]; !ok {
			byGuild[guildID] = &BoardConfig{GuildID: guildID, Channels: make(map[string]string), Messages: make(map[string]string)}
		}
		byGuild[guildID].Channels[shopType] = channelID
		byGuild[guildID].Messages[shopType] = messageID
	}

	var configs []BoardConfig
	for _, cfg := range byGuild {
		configs = append(configs, *cfg)
	}
	return configs, rows.Err()
}

// DeleteBoardConfig removes board config for a guild.
func (s *Store) DeleteBoardConfig(guildID string) error {
	_, err := s.db.Exec(`DELETE FROM board_messages WHERE guild_id = ?`, guildID)
	return err
}

// GetConfig returns a config value by key, or empty string if not found.
func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig stores a config key-value pair (upsert).
func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)`, key, value)
	return err
}

// GetAllConfig returns all config key-value pairs.
func (s *Store) GetAllConfig() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cfg := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		cfg[k] = v
	}
	return cfg, rows.Err()
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func scanSubscriptions(rows *sql.Rows) ([]Subscription, error) {
	var subs []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.GuildID, &sub.ItemID, &sub.ShopType); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

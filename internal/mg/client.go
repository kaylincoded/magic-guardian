package mg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	heartbeatInterval = 2 * time.Second
	reconnectBaseWait = 2 * time.Second
	reconnectMaxWait  = 60 * time.Second
)

// ClientConfig holds the configuration for the MG WebSocket client.
type ClientConfig struct {
	RoomID  string
	Version string
}

// Client manages the WebSocket connection to the Magic Garden server.
type Client struct {
	cfg           ClientConfig
	state         *ShopState
	conn          *websocket.Conn
	connMu        sync.Mutex
	onRestock     func([]StockChange)
	onStockChange func([]StockChange)
	onConnect     func()
	logger        *slog.Logger
}

// NewClient creates a new MG WebSocket client.
func NewClient(cfg ClientConfig, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		state:  NewShopState(),
		logger: logger,
	}
}

// OnRestock registers a callback for restock events (0 → N).
func (c *Client) OnRestock(fn func([]StockChange)) {
	c.onRestock = fn
}

// OnStockChange registers a callback for ANY stock change.
func (c *Client) OnStockChange(fn func([]StockChange)) {
	c.onStockChange = fn
}

// OnConnect registers a callback that fires after every successful Welcome.
func (c *Client) OnConnect(fn func()) {
	c.onConnect = fn
}

// State returns the current shop state.
func (c *Client) State() *ShopState {
	return c.state
}

// Run connects to the server and processes messages until ctx is cancelled.
// It automatically reconnects on disconnect, re-discovering fresh room params each time.
func (c *Client) Run(ctx context.Context) error {
	for {
		// Re-discover version and room on every connect (rooms expire quickly)
		version, roomID, err := DiscoverParams(ctx)
		if err != nil {
			c.logger.Warn("failed to discover params, using last known", "error", err)
		} else {
			c.cfg.Version = version
			c.cfg.RoomID = roomID
			c.logger.Info("discovered MG params", "version", version, "room", roomID)
		}

		err = c.connectAndListen(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.logger.Warn("disconnected from MG server", "error", err)
		wait := reconnectWait()
		c.logger.Info("reconnecting", "wait", wait)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

func (c *Client) authenticate(ctx context.Context, playerID string) error {
	authURL := fmt.Sprintf("https://magicgarden.gg/version/%s/api/rooms/%s/user/authenticate-web",
		c.cfg.Version, c.cfg.RoomID)

	body := map[string]string{"playerId": playerID}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://magicgarden.gg")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close()

	c.logger.Info("authenticate-web response", "status", resp.StatusCode)
	return nil
}

func (c *Client) connectAndListen(ctx context.Context) error {
	playerID := fmt.Sprintf("p_%s", randomID(16))
	wsURL := c.buildURLWithPlayer(playerID)

	// Authenticate before WebSocket connection
	if err := c.authenticate(ctx, playerID); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	c.logger.Info("connecting to MG", "url", wsURL)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()
	defer func() {
		conn.Close()
		c.connMu.Lock()
		c.conn = nil
		c.connMu.Unlock()
	}()

	c.logger.Info("connected to MG server")

	// Send initial messages to join the game
	c.sendJoinMessages(conn)

	// Start heartbeat
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	defer heartbeatCancel()
	go c.heartbeat(heartbeatCtx, conn)

	// Read messages (30s deadline resets on each message; server pings every ~5s)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		// Handle server text "ping" with "pong" response
		if string(raw) == "ping" {
			c.connMu.Lock()
			conn.WriteMessage(websocket.TextMessage, []byte("pong"))
			c.connMu.Unlock()
			continue
		}
		c.handleMessage(raw)
	}
}

func (c *Client) handleMessage(raw []byte) {
	var msg ServerMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.logger.Debug("failed to parse message", "error", err)
		return
	}

	switch msg.Type {
	case "Welcome":
		c.handleWelcome(msg.FullState)
	case "PartialState":
		if len(msg.Patches) > 0 {
			c.logger.Debug("partial state", "patches", len(msg.Patches), "first", msg.Patches[0].Path)
		}
		c.handlePartialState(msg.Patches)
	case "Config":
		c.logger.Debug("received config message")
	default:
		c.logger.Debug("unknown message type", "type", msg.Type)
	}
}

func (c *Client) handleWelcome(raw json.RawMessage) {
	var state WelcomeState
	if err := json.Unmarshal(raw, &state); err != nil {
		c.logger.Error("failed to parse welcome state", "error", err)
		return
	}

	if state.Child.Data.Shops == nil {
		c.logger.Warn("welcome state has no shops")
		return
	}

	// Snapshot old state before overwriting so we can diff
	oldShops := c.state.GetAllShops()

	c.state.SetFromWelcome(state.Child.Data.Shops)

	totalItems := 0
	for shopType, shop := range state.Child.Data.Shops {
		inStock := 0
		for _, item := range shop.Inventory {
			if item.InitialStock > 0 {
				inStock++
			}
		}
		totalItems += len(shop.Inventory)
		c.logger.Info("shop loaded",
			"shop", shopType,
			"items", len(shop.Inventory),
			"inStock", inStock,
			"restockIn", fmt.Sprintf("%.0fs", shop.SecondsUntilRestock),
		)
	}
	c.logger.Info("shop state initialized", "totalItems", totalItems)

	// Always refresh boards with current state
	if c.onConnect != nil {
		c.onConnect()
	}

	// Diff old vs new state — fire callbacks if stock changed
	if len(oldShops) > 0 {
		changes := diffShopState(oldShops, state.Child.Data.Shops)
		if len(changes) > 0 {
			c.logger.Info("stock changed on reconnect", "changes", len(changes))

			if c.onStockChange != nil {
				c.onStockChange(changes)
			}

			var restocks []StockChange
			for _, ch := range changes {
				if ch.NewStock > 0 {
					restocks = append(restocks, ch)
				}
			}
			if len(restocks) > 0 && c.onRestock != nil {
				c.onRestock(restocks)
			}
		}
	}
}

// diffShopState compares two shop snapshots and returns stock changes.
func diffShopState(old, new map[string]*Shop) []StockChange {
	var changes []StockChange
	for shopType, newShop := range new {
		oldShop, ok := old[shopType]
		if !ok {
			continue
		}
		// Build index of old items by ID
		oldStock := make(map[string]int)
		for _, item := range oldShop.Inventory {
			oldStock[item.ItemID()] = item.InitialStock
		}
		for _, item := range newShop.Inventory {
			id := item.ItemID()
			prev := oldStock[id]
			if prev != item.InitialStock {
				changes = append(changes, StockChange{
					ShopType: shopType,
					Item:     item,
					OldStock: prev,
					NewStock: item.InitialStock,
				})
			}
		}
	}
	return changes
}

func (c *Client) handlePartialState(patches []Patch) {
	changes := c.state.ApplyPatches(patches)
	if len(changes) == 0 {
		return
	}

	// Filter to only "now in stock" events (0 → N)
	var restocks []StockChange
	for _, ch := range changes {
		c.logger.Info("stock change",
			"shop", ch.ShopType,
			"item", ch.Item.ItemID(),
			"old", ch.OldStock,
			"new", ch.NewStock,
		)
		if ch.OldStock == 0 && ch.NewStock > 0 {
			restocks = append(restocks, ch)
		}
	}

	// Notify board updater about all changes
	if c.onStockChange != nil {
		c.onStockChange(changes)
	}

	// Notify DM engine about restocks only
	if len(restocks) > 0 && c.onRestock != nil {
		c.onRestock(restocks)
	}
}

func (c *Client) sendJoinMessages(conn *websocket.Conn) {
	msgs := []ClientMessage{
		{ScopePath: []string{"Room"}, Type: "VoteForGame", GameName: "Quinoa"},
		{ScopePath: []string{"Room"}, Type: "SetSelectedGame", GameName: "Quinoa"},
	}
	for _, m := range msgs {
		data, _ := json.Marshal(m)
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			c.logger.Error("failed to send join message", "error", err)
		}
	}
}

func (c *Client) heartbeat(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.connMu.Lock()
			if c.conn != conn {
				c.connMu.Unlock()
				return
			}
			ping := fmt.Sprintf(`{"scopePath":["Room","Quinoa"],"type":"Ping","id":%d}`, time.Now().UnixMilli())
			err := conn.WriteMessage(websocket.TextMessage, []byte(ping))
			c.connMu.Unlock()
			if err != nil {
				c.logger.Debug("heartbeat failed", "error", err)
				return
			}
		}
	}
}

func (c *Client) buildURLWithPlayer(playerID string) string {
	name := randomName()

	style := map[string]string{
		"color":            "White",
		"avatarBottom":     "Bottom_DefaultGray.png",
		"avatarMid":        "Mid_DefaultGray.png",
		"avatarTop":        "Top_DefaultGray.png",
		"avatarExpression": "Expression_Default.png",
		"name":             name,
	}
	styleJSON, _ := json.Marshal(style)

	params := url.Values{
		"surface":            {`"web"`},
		"platform":           {`"desktop"`},
		"playerId":           {fmt.Sprintf(`"%s"`, playerID)},
		"version":            {fmt.Sprintf(`"%s"`, c.cfg.Version)},
		"anonymousUserStyle": {string(styleJSON)},
		"source":             {`"router"`},
	}

	return fmt.Sprintf("wss://magicgarden.gg/version/%s/api/rooms/%s/connect?%s",
		c.cfg.Version, c.cfg.RoomID, params.Encode())
}

func randomID(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func randomName() string {
	adjectives := []string{"Bold", "Swift", "Quiet", "Gentle", "Bright"}
	nouns := []string{"Cherry", "Maple", "Willow", "Daisy", "Fern"}
	return adjectives[rand.Intn(len(adjectives))] + " " + nouns[rand.Intn(len(nouns))]
}

func reconnectWait() time.Duration {
	jitter := time.Duration(rand.Int63n(int64(reconnectBaseWait)))
	wait := reconnectBaseWait + jitter
	if wait > reconnectMaxWait {
		wait = reconnectMaxWait
	}
	return wait
}

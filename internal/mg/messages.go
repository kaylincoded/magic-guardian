package mg

import "encoding/json"

// ServerMessage is the top-level envelope for all server-to-client messages.
type ServerMessage struct {
	Type      string          `json:"type"`
	FullState json.RawMessage `json:"fullState,omitempty"`
	Patches   []Patch         `json:"patches,omitempty"`
	Config    json.RawMessage `json:"config,omitempty"`
}

// Patch represents a JSON Patch (RFC 6902) operation.
type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value json.Number `json:"value"`
}

// WelcomeState is the full state received in a Welcome message.
type WelcomeState struct {
	Scope string `json:"scope"`
	Data  struct {
		RoomID string `json:"roomId"`
	} `json:"data"`
	Child ChildState `json:"child"`
}

// ChildState contains the game-specific state (Quinoa = Magic Garden).
type ChildState struct {
	Scope string    `json:"scope"`
	Data  QuinoData `json:"data"`
}

// QuinoData holds the game data, including shops.
type QuinoData struct {
	CurrentTime float64          `json:"currentTime"`
	Shops       map[string]*Shop `json:"shops"`
}

// Shop represents a single shop (seed, tool, egg, decor).
type Shop struct {
	Inventory           []ShopItem `json:"inventory"`
	SecondsUntilRestock float64    `json:"secondsUntilRestock"`
	RestockCycle        float64    `json:"-"` // full cycle length in seconds, set after first timer reset
}

// ShopItem represents an item in a shop's inventory.
type ShopItem struct {
	ItemType     string `json:"itemType"`
	Species      string `json:"species,omitempty"`
	ToolID       string `json:"toolId,omitempty"`
	EggID        string `json:"eggId,omitempty"`
	DecorID      string `json:"decorId,omitempty"`
	InitialStock int    `json:"initialStock"`
}

// ItemID returns the canonical identifier for a shop item.
func (si ShopItem) ItemID() string {
	switch si.ItemType {
	case "Seed":
		return si.Species
	case "Tool":
		return si.ToolID
	case "Egg":
		return si.EggID
	case "Decor":
		return si.DecorID
	default:
		return ""
	}
}

// ClientMessage is a message sent from the client to the server.
type ClientMessage struct {
	ScopePath []string    `json:"scopePath"`
	Type      string      `json:"type"`
	GameName  string      `json:"gameName,omitempty"`
	Position  *Position   `json:"position,omitempty"`
	ItemIndex interface{} `json:"itemIndex,omitempty"`
}

// Position represents an x,y coordinate.
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

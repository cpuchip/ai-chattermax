// Package dnd is the client for a deployed dnd-tools service
// (github.com/cpuchip/dnd-tools): character sheets, campaigns, and lore for
// the D&D slash commands. The service resolves WHAT to roll (modifiers,
// dice expressions) — chattermax itself rolls, so the one-dice-fairness
// story holds: every roll happens server-side here, in the open.
package dnd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client calls the dnd-tools JSON API.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New builds a client; baseURL == "" disables the integration (Enabled()).
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether a dnd-tools service is configured.
func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// apiError carries the service's error message to the chat user.
type apiError struct{ Msg string }

func (e *apiError) Error() string { return e.Msg }

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("dnd-tools unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return &apiError{Msg: e.Error}
		}
		return fmt.Errorf("dnd-tools returned %s", resp.Status)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Character mirrors the dnd-tools sheet shape (fields the chat needs).
type Character struct {
	ID         int64          `json:"id"`
	Campaign   string         `json:"campaign"`
	Name       string         `json:"name"`
	Player     string         `json:"player"`
	Kind       string         `json:"kind"`
	Class      string         `json:"class"`
	Level      int            `json:"level"`
	HPMax      int            `json:"hp_max"`
	HPCurrent  int            `json:"hp_current"`
	AC         int            `json:"ac"`
	Conditions []string       `json:"conditions"`
	Raw        json.RawMessage `json:"-"`
}

// PlayerCharacter resolves the character a player runs in the room's campaign.
func (c *Client) PlayerCharacter(ctx context.Context, roomID, player string) (Character, error) {
	var ch Character
	err := c.do(ctx, "GET", "/api/rooms/"+url.PathEscape(roomID)+"/player/"+url.PathEscape(player), nil, &ch)
	return ch, err
}

// PlayerCharacterRaw is PlayerCharacter for pass-through consumers (the /char
// panel wants every sheet field, not the chat subset).
func (c *Client) PlayerCharacterRaw(ctx context.Context, roomID, player string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, "GET", "/api/rooms/"+url.PathEscape(roomID)+"/player/"+url.PathEscape(player), nil, &raw)
	return raw, err
}

// RoomCharacters returns the room campaign's roster (HP chips, panels).
// The raw JSON passes through so the frontend gets full sheets.
func (c *Client) RoomCharacters(ctx context.Context, roomID string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, "GET", "/api/rooms/"+url.PathEscape(roomID)+"/characters", nil, &raw)
	return raw, err
}

// CheckResult is a resolved d20 check.
type CheckResult struct {
	Character string `json:"character"`
	Label     string `json:"label"`
	Mod       int    `json:"mod"`
	Breakdown string `json:"breakdown"`
}

// Check resolves a skill/ability/save/initiative check for a character.
func (c *Client) Check(ctx context.Context, name, campaign, check string) (CheckResult, error) {
	var out CheckResult
	err := c.do(ctx, "POST", "/api/characters/"+url.PathEscape(name)+"/check?campaign="+url.QueryEscape(campaign),
		map[string]string{"check": check}, &out)
	return out, err
}

// AttackResult is a resolved weapon attack.
type AttackResult struct {
	Character string `json:"character"`
	Result    struct {
		Weapon     string `json:"weapon"`
		ToHit      int    `json:"to_hit"`
		DamageExpr string `json:"damage_expr"`
		DamageType string `json:"damage_type"`
		Breakdown  string `json:"breakdown"`
		DamageRoll string `json:"damage_roll"`
	} `json:"result"`
}

// Attack resolves a weapon attack (weapon "" = the sheet's first).
func (c *Client) Attack(ctx context.Context, name, campaign, weapon, target string) (AttackResult, error) {
	var out AttackResult
	err := c.do(ctx, "POST", "/api/characters/"+url.PathEscape(name)+"/attack?campaign="+url.QueryEscape(campaign),
		map[string]string{"weapon": weapon, "target": target}, &out)
	return out, err
}

// CastResult reports a spent (or free) spell slot.
type CastResult struct {
	Character      string `json:"character"`
	Spell          string `json:"spell"`
	Level          int    `json:"level"`
	SlotUsed       int    `json:"slot_used"`
	SlotsRemaining int    `json:"slots_remaining"`
	DamageRoll     string `json:"damage_roll"`
}

// Cast spends the slot and reports what's left (+ damage dice when known).
func (c *Client) Cast(ctx context.Context, name, campaign, spell string, slotLevel int) (CastResult, error) {
	var out CastResult
	err := c.do(ctx, "POST", "/api/characters/"+url.PathEscape(name)+"/cast?campaign="+url.QueryEscape(campaign),
		map[string]any{"spell": spell, "slot_level": slotLevel}, &out)
	return out, err
}

// HPResult is the sheet's HP after a delta.
type HPResult struct {
	Character string `json:"character"`
	HPCurrent int    `json:"hp_current"`
	HPMax     int    `json:"hp_max"`
	Delta     int    `json:"delta"`
}

// HP applies damage (negative) or healing (positive).
func (c *Client) HP(ctx context.Context, name, campaign string, delta int) (HPResult, error) {
	var out HPResult
	err := c.do(ctx, "POST", "/api/characters/"+url.PathEscape(name)+"/hp?campaign="+url.QueryEscape(campaign),
		map[string]int{"delta": delta}, &out)
	return out, err
}

// CampaignByRoom resolves the campaign bound to a room.
func (c *Client) CampaignByRoom(ctx context.Context, roomID string) (string, error) {
	var out struct {
		Name string `json:"name"`
	}
	err := c.do(ctx, "GET", "/api/rooms/"+url.PathEscape(roomID)+"/campaign", nil, &out)
	return out.Name, err
}

// CharacterByName fetches a full sheet as raw JSON (the /char panel).
func (c *Client) CharacterByName(ctx context.Context, name, campaign string) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, "GET", "/api/characters/"+url.PathEscape(name)+"?campaign="+url.QueryEscape(campaign), nil, &raw)
	return raw, err
}

// Patch applies a partial sheet edit and returns the updated sheet.
func (c *Client) Patch(ctx context.Context, name, campaign string, patch json.RawMessage) (json.RawMessage, error) {
	var raw json.RawMessage
	err := c.do(ctx, "PATCH", "/api/characters/"+url.PathEscape(name)+"?campaign="+url.QueryEscape(campaign),
		json.RawMessage(patch), &raw)
	return raw, err
}

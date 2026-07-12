package pixellab

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Response from PixelLab API
type Response struct {
	Usage struct {
		USD float64 `json:"usd"`
	} `json:"usage"`
	Image struct {
		Base64 string `json:"base64"`
	} `json:"image"`
}

// Balance response from PixelLab
type Balance struct {
	Type string  `json:"type"`
	USD  float64 `json:"usd"`
}

// Client for PixelLab image generation
type Client struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// NewClient creates a new PixelLab client
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: "https://api.pixellab.ai/v1",
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// GetBalance checks API balance
func (c *Client) GetBalance() (*Balance, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/balance", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var balance Balance
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		return nil, err
	}
	return &balance, nil
}

// base64Image is the wire shape PixelLab expects for init/style/mask images.
type base64Image struct {
	Type   string `json:"type"`
	Base64 string `json:"base64"`
	Format string `json:"format"`
}

func newBase64Image(pngBytes []byte) *base64Image {
	return &base64Image{
		Type:   "base64",
		Base64: base64.StdEncoding.EncodeToString(pngBytes),
		Format: "png",
	}
}

// GenerateOptions are the knobs for a single image generation. Zero values fall
// back to sensible sprite defaults (32×32, transparent, black outline).
type GenerateOptions struct {
	Description         string
	NegativeDescription string
	Model               string // "bitforge" (supports style) or "pixflux"
	Width               int    // default 32
	Height              int    // default 32

	// InitImage seeds generation from an existing sprite (raw PNG bytes) so the
	// result keeps its silhouette — the mechanism for uniform families. Strength
	// is 0–1000 (higher = closer to the init). Supported by both models.
	InitImage         []byte
	InitImageStrength int // default 300 when InitImage set

	// StyleImage transfers the look of a reference sprite (raw PNG bytes).
	// bitforge only. StyleStrength is 0–100; ExtraGuidanceScale ~0–20.
	StyleImage         []byte
	StyleStrength      float64
	ExtraGuidanceScale float64

	Seed int // 0 = random per call
}

// Generate creates one pixel-art image with full control over init/style images.
func (c *Client) Generate(opts GenerateOptions) (*Response, error) {
	w, h := opts.Width, opts.Height
	if w == 0 {
		w = 32
	}
	if h == 0 {
		h = 32
	}

	var endpoint string
	switch opts.Model {
	case "bitforge":
		endpoint = "/generate-image-bitforge"
	case "pixflux":
		endpoint = "/generate-image-pixflux"
	default:
		return nil, fmt.Errorf("unsupported model: %s", opts.Model)
	}

	payload := map[string]interface{}{
		"description":   opts.Description,
		"image_size":    map[string]int{"width": w, "height": h},
		"no_background": true,
		"detail":        "highly detailed",
		"outline":       "single color black outline",
	}
	if opts.NegativeDescription != "" {
		payload["negative_description"] = opts.NegativeDescription
	}
	if opts.Seed != 0 {
		payload["seed"] = opts.Seed
	}
	if len(opts.InitImage) > 0 {
		strength := opts.InitImageStrength
		if strength == 0 {
			strength = 300
		}
		payload["init_image"] = newBase64Image(opts.InitImage)
		payload["init_image_strength"] = strength
	}
	// style_image is a bitforge-only feature; silently skip it for pixflux.
	if len(opts.StyleImage) > 0 && opts.Model == "bitforge" {
		payload["style_image"] = newBase64Image(opts.StyleImage)
		if opts.StyleStrength != 0 {
			payload["style_strength"] = opts.StyleStrength
		}
		if opts.ExtraGuidanceScale != 0 {
			payload["extra_guidance_scale"] = opts.ExtraGuidanceScale
		}
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GenerateImage is the original text-only entry point, preserved for the codex
// editor's existing generate/accept flow.
func (c *Client) GenerateImage(description, negativePrompt string, model string) (*Response, error) {
	return c.Generate(GenerateOptions{
		Description:         description,
		NegativeDescription: negativePrompt,
		Model:               model,
	})
}

// GeneratePrompt creates a fantasy prompt from item properties
func GeneratePrompt(name, description, rarity string) string {
	// Priority 1: Use regular description if available
	if description != "" && len(description) > 10 {
		return description
	}

	// Fallback: Generate from item name and rarity
	rarityLower := strings.ToLower(rarity)
	var rarityDesc string
	switch rarityLower {
	case "common":
		rarityDesc = "simple, basic"
	case "uncommon":
		rarityDesc = "well-crafted, slightly ornate"
	case "rare":
		rarityDesc = "ornate, decorated"
	case "very rare":
		rarityDesc = "highly ornate, magical aura"
	case "legendary":
		rarityDesc = "legendary, glowing, magical effects"
	default:
		rarityDesc = "well-made"
	}

	return fmt.Sprintf("%s %s", rarityDesc, name)
}

// NegativePrompt returns the standard negative prompt for pixel art
func NegativePrompt() string {
	return "blurry, fuzzy, soft, antialiased, smooth, low quality, modern, realistic, photograph, 3d render, low resolution, text, letters, words, people, characters, faces, anime, cartoon"
}

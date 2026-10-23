// Package openai is the codex's client for OpenAI's image API, used for the
// large scene and location art PixelLab can't make (its canvas tops out at
// 400×400). Text-only prompts go to /images/generations; prompts with
// reference images (existing scenes, to match the style) go to /images/edits.
package openai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"
)

// Client for OpenAI image generation.
type Client struct {
	APIKey  string
	BaseURL string
	Client  *http.Client
}

// NewClient creates a new OpenAI client.
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
		Client:  &http.Client{Timeout: 5 * time.Minute}, // large high-quality images are slow
	}
}

// GenerateOptions describes one image.
type GenerateOptions struct {
	Model   string // e.g. "gpt-image-2"
	Prompt  string // required
	Width   int    // multiples of 16 (gpt-image-2 takes custom sizes)
	Height  int
	Quality string   // "low" | "medium" | "high"; empty → the API default
	Refs    [][]byte // PNG reference images; any → /images/edits
}

// Usage is the token count the API reports for one image.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type response struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Usage Usage `json:"usage"`
}

// Generate makes one image and returns its PNG bytes.
func (c *Client) Generate(opts GenerateOptions) ([]byte, Usage, error) {
	if opts.Prompt == "" {
		return nil, Usage{}, fmt.Errorf("prompt is required")
	}
	size := fmt.Sprintf("%dx%d", opts.Width, opts.Height)

	var req *http.Request
	var err error
	if len(opts.Refs) == 0 {
		body := map[string]any{
			"model":  opts.Model,
			"prompt": opts.Prompt,
			"size":   size,
			"n":      1,
		}
		if opts.Quality != "" {
			body["quality"] = opts.Quality
		}
		raw, _ := json.Marshal(body)
		req, err = http.NewRequest("POST", c.BaseURL+"/images/generations", bytes.NewReader(raw))
		if err != nil {
			return nil, Usage{}, err
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fields := map[string]string{"model": opts.Model, "prompt": opts.Prompt, "size": size, "n": "1"}
		if opts.Quality != "" {
			fields["quality"] = opts.Quality
		}
		for k, v := range fields {
			if err := mw.WriteField(k, v); err != nil {
				return nil, Usage{}, err
			}
		}
		for i, ref := range opts.Refs {
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="%s"`, fmt.Sprintf("ref%d.png", i+1)))
			h.Set("Content-Type", "image/png")
			part, err := mw.CreatePart(h)
			if err != nil {
				return nil, Usage{}, err
			}
			if _, err := part.Write(ref); err != nil {
				return nil, Usage{}, err
			}
		}
		if err := mw.Close(); err != nil {
			return nil, Usage{}, err
		}
		req, err = http.NewRequest("POST", c.BaseURL+"/images/edits", &buf)
		if err != nil {
			return nil, Usage{}, err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, Usage{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Usage{}, err
	}
	if resp.StatusCode != 200 {
		return nil, Usage{}, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	var out response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, Usage{}, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Data) == 0 || out.Data[0].B64JSON == "" {
		return nil, out.Usage, fmt.Errorf("no image in response")
	}
	img, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
	if err != nil {
		return nil, out.Usage, fmt.Errorf("decode image: %w", err)
	}
	return img, out.Usage, nil
}

// Package imagegen is the standalone image batch tool: it reads a job spec and
// writes generated *candidates* into <img_dir>/_candidates/<id>/. It never
// overwrites a live image — promotion happens in the codex UI (sprites) or by
// hand (scenes). Two providers: PixelLab for pixel-art sprites (the default),
// and OpenAI for the large scene and location art PixelLab can't make.
package imagegen

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pubkey-quest/cmd/codex/openai"
	"pubkey-quest/cmd/codex/pixellab"
)

const itemsImgDir = "www/res/img/items"

// Spec is the batch file: top-level defaults plus a list of per-item jobs.
type Spec struct {
	Provider string `json:"provider"` // "pixellab" (default) | "openai"
	Model    string `json:"model"`    // default model for jobs ("bitforge"|"pixflux"; openai: "gpt-image-2")
	Quality  string `json:"quality"`  // openai: "low" | "medium" | "high"
	Size     int    `json:"size"`     // default sprite size (px, square); 0 → 32
	Width    int    `json:"width"`    // overrides Size for non-square sprites (e.g. 96×72 monsters)
	Height   int    `json:"height"`
	ImgDir   string `json:"img_dir"`  // sprite folder for ids + candidates; default www/res/img/items
	Count    int    `json:"count"`    // default variants per job; 0 → 3
	Negative string `json:"negative"` // default negative prompt
	Jobs     []Job  `json:"jobs"`
}

// Job describes one item's generation. Anchor/Style may be a bare item id
// (resolved to www/res/img/items/<id>.png) or a path to a PNG.
type Job struct {
	ID            string   `json:"id"`             // target item id (candidate dir name)
	Prompt        string   `json:"prompt"`         // required description
	Negative      string   `json:"negative"`       // overrides Spec.Negative
	Model         string   `json:"model"`          // overrides Spec.Model
	Count         int      `json:"count"`          // overrides Spec.Count
	Seeds         []int    `json:"seeds"`          // explicit seeds; else 1..count
	Anchor        string   `json:"anchor"`         // init_image (id or path) — keeps silhouette
	InitStrength  int      `json:"init_strength"`  // 0–1000; 0 → client default (300)
	Style         string   `json:"style"`          // style_image (id or path) — bitforge only
	StyleStrength float64  `json:"style_strength"` // 0–100
	Refs          []string `json:"refs"`           // openai: reference images (paths) to match, e.g. existing scenes
}

// ReadSpec loads a batch spec.
func ReadSpec(specPath string) (*Spec, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("parse spec %s: %w", specPath, err)
	}
	if len(spec.Jobs) == 0 {
		return nil, fmt.Errorf("spec has no jobs")
	}
	return &spec, nil
}

// RunBatch generates all PixelLab candidates described by the spec.
func RunBatch(client *pixellab.Client, spec *Spec) error {

	// Report balance for visibility, but don't gate on it: monthly-plan accounts
	// always report $0.00 yet still generate. A genuinely empty pay-as-you-go
	// account will simply error per-image, which we surface below.
	if bal, err := client.GetBalance(); err != nil {
		fmt.Printf("⚠️  could not read balance (%v) — continuing\n", err)
	} else {
		fmt.Printf("💳 PixelLab balance: $%.4f (ignored; monthly plans report $0)\n", bal.USD)
	}

	imgDir := firstNonEmpty(spec.ImgDir, itemsImgDir)
	width := firstNonZero(spec.Width, spec.Size)
	height := firstNonZero(spec.Height, spec.Size)

	var total float64
	var made, skipped, failed int
	for _, job := range spec.Jobs {
		if job.ID == "" || job.Prompt == "" {
			fmt.Printf("⚠️  skipping job with missing id/prompt: %+v\n", job)
			continue
		}
		model := firstNonEmpty(job.Model, spec.Model, "bitforge")
		negative := firstNonEmpty(job.Negative, spec.Negative, pixellab.NegativePrompt())
		count := firstNonZero(job.Count, spec.Count, 3)
		seeds := job.Seeds
		if len(seeds) == 0 {
			for s := 1; s <= count; s++ {
				seeds = append(seeds, s)
			}
		}

		initImg, err := resolveRef(imgDir, job.Anchor)
		if err != nil {
			fmt.Printf("❌ %s: anchor %q: %v\n", job.ID, job.Anchor, err)
			failed++
			continue
		}
		styleImg, err := resolveRef(imgDir, job.Style)
		if err != nil {
			fmt.Printf("❌ %s: style %q: %v\n", job.ID, job.Style, err)
			failed++
			continue
		}

		outDir := filepath.Join(imgDir, "_candidates", job.ID)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", outDir, err)
		}

		fmt.Printf("🎨 %s — %d variant(s) via %s\n", job.ID, len(seeds), model)
		for _, seed := range seeds {
			outFile := filepath.Join(outDir, fmt.Sprintf("%s-s%d.png", model, seed))
			if _, statErr := os.Stat(outFile); statErr == nil {
				fmt.Printf("   ↷ seed %d exists, skipping\n", seed)
				skipped++
				continue
			}
			resp, genErr := generateWithRetry(client, pixellab.GenerateOptions{
				Description:         job.Prompt,
				NegativeDescription: negative,
				Model:               model,
				Width:               width,
				Height:              height,
				InitImage:           initImg,
				InitImageStrength:   job.InitStrength,
				StyleImage:          styleImg,
				StyleStrength:       job.StyleStrength,
				Seed:                seed,
			})
			if genErr != nil {
				fmt.Printf("   ❌ seed %d: %v\n", seed, genErr)
				failed++
				continue
			}
			imgBytes, decErr := base64.StdEncoding.DecodeString(resp.Image.Base64)
			if decErr != nil {
				fmt.Printf("   ❌ seed %d: decode: %v\n", seed, decErr)
				failed++
				continue
			}
			if err := os.WriteFile(outFile, imgBytes, 0644); err != nil {
				return fmt.Errorf("write %s: %w", outFile, err)
			}
			total += resp.Usage.USD
			made++
			fmt.Printf("   ✅ seed %d → %s ($%.4f)\n", seed, outFile, resp.Usage.USD)
		}
	}

	fmt.Printf("\n✅ done — %d generated, %d skipped, %d failed. Spent $%.4f.\n", made, skipped, failed, total)
	fmt.Printf("👉 Review & accept in the codex item editor (candidates land under _candidates/<id>/).\n")
	if failed > 0 {
		return fmt.Errorf("%d image(s) failed", failed)
	}
	return nil
}

// RunOpenAIBatch generates OpenAI candidates: count variants per job, written
// as <model>-v<n>.png (OpenAI has no seeds; an existing file is skipped, so a
// rerun only fills gaps). Width and height go straight to the API, so they
// must be multiples of 16 (scenes: 1408×736).
func RunOpenAIBatch(client *openai.Client, spec *Spec) error {
	imgDir := firstNonEmpty(spec.ImgDir, itemsImgDir)
	width := firstNonZero(spec.Width, spec.Size, 1408)
	height := firstNonZero(spec.Height, spec.Size, 736)

	var in, out, made, skipped, failed int
	for _, job := range spec.Jobs {
		if job.ID == "" || job.Prompt == "" {
			fmt.Printf("⚠️  skipping job with missing id/prompt: %+v\n", job)
			continue
		}
		model := firstNonEmpty(job.Model, spec.Model, "gpt-image-2")
		count := firstNonZero(job.Count, spec.Count, 2)
		refs, err := readRefs(job.Refs)
		if err != nil {
			fmt.Printf("❌ %s: %v\n", job.ID, err)
			failed++
			continue
		}

		outDir := filepath.Join(imgDir, "_candidates", job.ID)
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", outDir, err)
		}
		fmt.Printf("🎨 %s — %d variant(s) via %s (%dx%d, %s)\n", job.ID, count, model, width, height, firstNonEmpty(spec.Quality, "default quality"))
		for v := 1; v <= count; v++ {
			outFile := filepath.Join(outDir, fmt.Sprintf("%s-v%d.png", model, v))
			if _, statErr := os.Stat(outFile); statErr == nil {
				fmt.Printf("   ↷ v%d exists, skipping\n", v)
				skipped++
				continue
			}
			img, usage, err := client.Generate(openai.GenerateOptions{
				Model:   model,
				Prompt:  job.Prompt,
				Width:   width,
				Height:  height,
				Quality: spec.Quality,
				Refs:    refs,
			})
			if err != nil {
				fmt.Printf("   ❌ v%d: %v\n", v, err)
				failed++
				continue
			}
			if err := os.WriteFile(outFile, img, 0644); err != nil {
				return fmt.Errorf("write %s: %w", outFile, err)
			}
			in += usage.InputTokens
			out += usage.OutputTokens
			made++
			fmt.Printf("   ✅ v%d → %s (%d in / %d out tokens)\n", v, outFile, usage.InputTokens, usage.OutputTokens)
		}
	}

	fmt.Printf("\n✅ done — %d generated, %d skipped, %d failed. Tokens: %d in / %d out (the OpenAI usage page has the cost).\n", made, skipped, failed, in, out)
	if failed > 0 {
		return fmt.Errorf("%d image(s) failed", failed)
	}
	return nil
}

// readRefs loads reference images by path.
func readRefs(paths []string) ([][]byte, error) {
	var refs [][]byte
	for _, r := range paths {
		b, err := os.ReadFile(r)
		if err != nil {
			return nil, fmt.Errorf("ref %q: %w", r, err)
		}
		refs = append(refs, b)
	}
	return refs, nil
}

// resolveRef turns an empty/id/path reference into raw PNG bytes. Empty → nil.
// A bare id resolves to <imgDir>/<id>.png.
func resolveRef(imgDir, ref string) ([]byte, error) {
	if ref == "" {
		return nil, nil
	}
	path := ref
	if !strings.Contains(ref, "/") && !strings.Contains(ref, "\\") && !strings.HasSuffix(strings.ToLower(ref), ".png") {
		path = filepath.Join(imgDir, ref+".png")
	}
	return os.ReadFile(path)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// generateWithRetry waits and retries when PixelLab reports too many
// concurrent jobs (HTTP 429), so several batches can run side by side.
func generateWithRetry(client *pixellab.Client, opts pixellab.GenerateOptions) (*pixellab.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 12; attempt++ {
		resp, err := client.Generate(opts)
		if err == nil || !strings.Contains(err.Error(), "API error 429") {
			return resp, err
		}
		lastErr = err
		time.Sleep(time.Duration(5+attempt*5) * time.Second)
	}
	return nil, lastErr
}

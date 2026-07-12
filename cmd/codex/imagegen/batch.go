// Package imagegen is the standalone PixelLab batch tool: it reads a job spec
// and writes generated sprite *candidates* into the codex candidate gallery
// (www/res/img/items/_candidates/<id>/). It never overwrites a live sprite —
// promotion happens in the codex UI. This is an internal dev tool, decoupled
// from the item editor, and expected to be dropped eventually.
package imagegen

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/cmd/codex/pixellab"
)

const itemsImgDir = "www/res/img/items"

// Spec is the batch file: top-level defaults plus a list of per-item jobs.
type Spec struct {
	Model    string `json:"model"`    // default model for jobs ("bitforge"|"pixflux")
	Size     int    `json:"size"`     // default sprite size (px, square); 0 → 32
	Count    int    `json:"count"`    // default variants per job; 0 → 3
	Negative string `json:"negative"` // default negative prompt
	Jobs     []Job  `json:"jobs"`
}

// Job describes one item's generation. Anchor/Style may be a bare item id
// (resolved to www/res/img/items/<id>.png) or a path to a PNG.
type Job struct {
	ID            string  `json:"id"`             // target item id (candidate dir name)
	Prompt        string  `json:"prompt"`         // required description
	Negative      string  `json:"negative"`       // overrides Spec.Negative
	Model         string  `json:"model"`          // overrides Spec.Model
	Count         int     `json:"count"`          // overrides Spec.Count
	Seeds         []int   `json:"seeds"`          // explicit seeds; else 1..count
	Anchor        string  `json:"anchor"`         // init_image (id or path) — keeps silhouette
	InitStrength  int     `json:"init_strength"`  // 0–1000; 0 → client default (300)
	Style         string  `json:"style"`          // style_image (id or path) — bitforge only
	StyleStrength float64 `json:"style_strength"` // 0–100
}

// RunBatch generates all candidates described by the spec at specPath.
func RunBatch(client *pixellab.Client, specPath string) error {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return fmt.Errorf("parse spec %s: %w", specPath, err)
	}
	if len(spec.Jobs) == 0 {
		return fmt.Errorf("spec has no jobs")
	}

	// Report balance for visibility, but don't gate on it: monthly-plan accounts
	// always report $0.00 yet still generate. A genuinely empty pay-as-you-go
	// account will simply error per-image, which we surface below.
	if bal, err := client.GetBalance(); err != nil {
		fmt.Printf("⚠️  could not read balance (%v) — continuing\n", err)
	} else {
		fmt.Printf("💳 PixelLab balance: $%.4f (ignored; monthly plans report $0)\n", bal.USD)
	}

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

		initImg, err := resolveRef(job.Anchor)
		if err != nil {
			fmt.Printf("❌ %s: anchor %q: %v\n", job.ID, job.Anchor, err)
			failed++
			continue
		}
		styleImg, err := resolveRef(job.Style)
		if err != nil {
			fmt.Printf("❌ %s: style %q: %v\n", job.ID, job.Style, err)
			failed++
			continue
		}

		outDir := filepath.Join(itemsImgDir, "_candidates", job.ID)
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
			resp, genErr := client.Generate(pixellab.GenerateOptions{
				Description:         job.Prompt,
				NegativeDescription: negative,
				Model:               model,
				Width:               spec.Size,
				Height:              spec.Size,
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

// resolveRef turns an empty/id/path reference into raw PNG bytes. Empty → nil.
func resolveRef(ref string) ([]byte, error) {
	if ref == "" {
		return nil, nil
	}
	path := ref
	if !strings.Contains(ref, "/") && !strings.Contains(ref, "\\") && !strings.HasSuffix(strings.ToLower(ref), ".png") {
		path = filepath.Join(itemsImgDir, ref+".png")
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

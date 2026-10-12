package game

import (
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScaleSquareCropsAndShrinks(t *testing.T) {
	// 200×100: left half red, right half blue → centre crop is 100×100 spanning both.
	src := image.NewNRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			c := color.NRGBA{R: 255, A: 255}
			if x >= 100 {
				c = color.NRGBA{B: 255, A: 255}
			}
			src.Set(x, y, c)
		}
	}
	out := scaleSquare(src, 64)
	if b := out.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("size %v, want 64×64", b)
	}
	if r, _, _, _ := out.At(2, 32).RGBA(); r>>8 < 200 {
		t.Errorf("left edge should be red, got r=%d", r>>8)
	}
	if _, _, b, _ := out.At(61, 32).RGBA(); b>>8 < 200 {
		t.Errorf("right edge should be blue, got b=%d", b>>8)
	}
}

func TestValidPortraitURL(t *testing.T) {
	for _, u := range []string{"https://blossom.example/abc.png", "https://cdn.example.com/a/b.webp?x=1"} {
		if err := validPortraitURL(u); err != nil {
			t.Errorf("%s rejected: %v", u, err)
		}
	}
	for _, u := range []string{"http://example.com/a.png", "javascript:alert(1)", "ftp://x/y", "https://", "https://x/" + strings.Repeat("a", 600)} {
		if validPortraitURL(u) == nil {
			t.Errorf("%s accepted", u)
		}
	}
}

func TestPortraitFetchRefusesInternalAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secret"))
	}))
	defer srv.Close()
	if _, err := fetchPortrait(srv.URL); err == nil || !strings.Contains(err.Error(), "internal address") {
		t.Errorf("fetching %s should be refused, got %v", srv.URL, err)
	}
}

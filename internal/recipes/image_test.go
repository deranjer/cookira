package recipes

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func solid(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeJPG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func dims(t *testing.T, data []byte) (int, int, string) {
	t.Helper()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height, format
}

func TestProcessImageResizesAndConvertsToJPEG(t *testing.T) {
	full, thumb, err := processImage(encodePNG(t, solid(3200, 2400, color.RGBA{200, 80, 20, 255})))
	if err != nil {
		t.Fatal(err)
	}
	w, h, f := dims(t, full)
	if f != "jpeg" || w != fullMaxEdge || h != 1200 {
		t.Errorf("full = %dx%d %s, want %dx1200 jpeg", w, h, f, fullMaxEdge)
	}
	w, h, f = dims(t, thumb)
	if f != "jpeg" || w != thumbMaxEdge || h != 480 {
		t.Errorf("thumb = %dx%d %s, want %dx480 jpeg", w, h, f, thumbMaxEdge)
	}
	if len(thumb) >= len(full) {
		t.Errorf("thumb (%d bytes) not smaller than full (%d)", len(thumb), len(full))
	}
}

func TestProcessImageNeverUpscales(t *testing.T) {
	full, thumb, err := processImage(encodeJPG(t, solid(300, 200, color.White)))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"full": full, "thumb": thumb} {
		if w, h, _ := dims(t, data); w != 300 || h != 200 {
			t.Errorf("%s = %dx%d, want 300x200", name, w, h)
		}
	}
}

func TestProcessImageTallImageFitsLongEdge(t *testing.T) {
	_, thumb, err := processImage(encodeJPG(t, solid(1000, 3000, color.White)))
	if err != nil {
		t.Fatal(err)
	}
	if w, h, _ := dims(t, thumb); h != thumbMaxEdge || w != 213 {
		t.Errorf("thumb = %dx%d, want 213x%d", w, h, thumbMaxEdge)
	}
}

func TestProcessImageFlattensTransparencyOntoWhite(t *testing.T) {
	full, _, err := processImage(encodePNG(t, solid(40, 40, color.NRGBA{0, 0, 0, 0})))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(20, 20).RGBA()
	if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Errorf("transparent pixel became %d,%d,%d, want white", r>>8, g>>8, b>>8)
	}
}

func TestProcessImageRejectsNonImages(t *testing.T) {
	for name, data := range map[string][]byte{
		"text":      []byte("hello, definitely not an image"),
		"html":      []byte("<html><script>alert(1)</script></html>"),
		"truncated": encodePNG(t, solid(50, 50, color.White))[:30],
		"empty":     nil,
	} {
		if _, _, err := processImage(data); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "93.184.216.34": true, "2606:4700::1111": true,
		"127.0.0.1": false, "::1": false, "10.0.0.5": false, "192.168.1.1": false,
		"172.16.0.1": false, "169.254.169.254": false, "100.64.0.1": false,
		"0.0.0.0": false, "224.0.0.1": false, "fe80::1": false, "fc00::1": false,
		"::ffff:127.0.0.1": false, "::ffff:10.1.2.3": false,
	} {
		if got := isPublicIP(net.ParseIP(ip)); got != want {
			t.Errorf("isPublicIP(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestFetcherRefusesLocalAddresses(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()

	_, err := newFetcher(false).fetch(context.Background(), srv.URL)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if hit {
		t.Error("request reached the loopback server")
	}
	for _, u := range []string{"file:///etc/passwd", "ftp://example.com/x.png", "not a url", ""} {
		if _, err := newFetcher(false).fetch(context.Background(), u); !errors.Is(err, ErrInvalid) {
			t.Errorf("fetch(%q) err = %v, want ErrInvalid", u, err)
		}
	}
}

func TestFetcherDownloadsAndCapsSize(t *testing.T) {
	img := encodeJPG(t, solid(20, 20, color.White))
	mux := http.NewServeMux()
	mux.HandleFunc("/ok.jpg", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(img) })
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	mux.HandleFunc("/huge", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, MaxImageBytes+10))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok.jpg", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newFetcher(true) // allow loopback so the test server is reachable
	got, err := f.fetch(context.Background(), srv.URL+"/ok.jpg")
	if err != nil || !bytes.Equal(got, img) {
		t.Fatalf("fetch ok.jpg: %v", err)
	}
	if got, err = f.fetch(context.Background(), srv.URL+"/redirect"); err != nil || !bytes.Equal(got, img) {
		t.Errorf("redirect: %v", err)
	}
	for _, path := range []string{"/missing", "/huge"} {
		if _, err := f.fetch(context.Background(), srv.URL+path); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", path, err)
		}
	}
}

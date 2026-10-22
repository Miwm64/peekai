package capture

import (
	"image"
	"testing"
)

func TestRectFor(t *testing.T) {
	primary := image.Rect(0, 0, 1920, 1080)
	right := image.Rect(1920, 0, 3840, 1440)
	displays := []image.Rectangle{primary, right}

	onRight := image.Pt(2500, 700)
	tests := []struct {
		name, display string
		mouse         image.Point
		mouseOK       bool
		want          image.Rectangle
		wantErr       bool
	}{
		{"mouse follows mouse", "mouse", onRight, true, right, false},
		{"mouse on primary", "mouse", image.Pt(10, 10), true, primary, false},
		{"mouse unknown", "mouse", onRight, false, primary, false},
		{"mouse off-screen", "mouse", image.Pt(-500, 0), true, primary, false},
		{"main ignores mouse", "main", onRight, true, primary, false},
		{"all", "all", onRight, true, image.Rect(0, 0, 3840, 1440), false},
		{"index 0", "0", onRight, true, primary, false},
		{"index 1", "1", image.Pt(10, 10), true, right, false},
		{"index too big", "2", onRight, true, image.Rectangle{}, true},
		{"negative", "-1", onRight, true, image.Rectangle{}, true},
		{"name", "left", onRight, true, image.Rectangle{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rectFor(tt.display, displays, tt.mouse, tt.mouseOK)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("rect = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectForNoDisplays(t *testing.T) {
	if _, err := rectFor("mouse", nil, image.Point{}, false); err == nil {
		t.Error("want error with no displays")
	}
}

func TestDownscale(t *testing.T) {
	tests := []struct {
		name         string
		w, h, maxPx  int
		wantW, wantH int
	}{
		{"landscape", 3200, 1800, 1600, 1600, 900},
		{"portrait", 1000, 2000, 500, 250, 500},
		{"already small", 800, 600, 1600, 800, 600},
		{"disabled", 4000, 3000, 0, 4000, 3000},
		{"thin keeps 1px", 5000, 2, 100, 100, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Downscale(image.NewRGBA(image.Rect(0, 0, tt.w, tt.h)), tt.maxPx).Bounds()
			if got.Dx() != tt.wantW || got.Dy() != tt.wantH {
				t.Errorf("size = %dx%d, want %dx%d", got.Dx(), got.Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

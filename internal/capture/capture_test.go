package capture

import (
	"image"
	"testing"
)

func TestRectFor(t *testing.T) {
	primary := image.Rect(0, 0, 1920, 1080)
	right := image.Rect(1920, 0, 3840, 1440)
	displays := []image.Rectangle{primary, right}

	tests := []struct {
		display string
		want    image.Rectangle
		wantErr bool
	}{
		{"active", primary, false},
		{"all", image.Rect(0, 0, 3840, 1440), false},
		{"0", primary, false},
		{"1", right, false},
		{"2", image.Rectangle{}, true},
		{"-1", image.Rectangle{}, true},
		{"left", image.Rectangle{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.display, func(t *testing.T) {
			got, err := rectFor(tt.display, displays)
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
	if _, err := rectFor("active", nil); err == nil {
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

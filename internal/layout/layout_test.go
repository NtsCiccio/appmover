package layout

import "testing"

func TestCentered(t *testing.T) {
	tests := []struct {
		name                       string
		w, h                       int32
		workX, workY, workW, workH int32
		dpiScale                   float64
		want                       Rect
	}{
		{
			name: "fits, centers in work area",
			w:    800, h: 600,
			workX: 0, workY: 0, workW: 1920, workH: 1040,
			dpiScale: 0,
			want:     Rect{X: 560, Y: 220, W: 800, H: 600},
		},
		{
			name: "work area offset (second monitor)",
			w:    800, h: 600,
			workX: 1920, workY: 0, workW: 1920, workH: 1040,
			dpiScale: 0,
			want:     Rect{X: 2480, Y: 220, W: 800, H: 600},
		},
		{
			name: "larger than work area, clamped on both axes",
			w:    3000, h: 2000,
			workX: 0, workY: 0, workW: 1920, workH: 1040,
			dpiScale: 0,
			want:     Rect{X: 0, Y: 0, W: 1920, H: 1040},
		},
		{
			name: "dpiScale of 1 is a no-op",
			w:    800, h: 600,
			workX: 0, workY: 0, workW: 1920, workH: 1040,
			dpiScale: 1,
			want:     Rect{X: 560, Y: 220, W: 800, H: 600},
		},
		{
			name: "dpiScale of 0 is treated as no-op",
			w:    800, h: 600,
			workX: 0, workY: 0, workW: 1920, workH: 1040,
			dpiScale: 0,
			want:     Rect{X: 560, Y: 220, W: 800, H: 600},
		},
		{
			name: "dpiScale upscales then clamps to work area",
			w:    800, h: 600,
			workX: 0, workY: 0, workW: 1000, workH: 1000,
			dpiScale: 1.5,
			want:     Rect{X: 0, Y: 50, W: 1000, H: 900},
		},
		{
			name: "negative dpiScale is treated as no-op",
			w:    800, h: 600,
			workX: 0, workY: 0, workW: 1920, workH: 1040,
			dpiScale: -1,
			want:     Rect{X: 560, Y: 220, W: 800, H: 600},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Centered(tt.w, tt.h, tt.workX, tt.workY, tt.workW, tt.workH, tt.dpiScale)
			if got != tt.want {
				t.Errorf("Centered(%d,%d, %d,%d,%d,%d, %v) = %+v, want %+v",
					tt.w, tt.h, tt.workX, tt.workY, tt.workW, tt.workH, tt.dpiScale, got, tt.want)
			}
		})
	}
}

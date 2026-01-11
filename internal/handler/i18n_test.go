package handler

import "testing"

func TestLocalizeFixedTitle(t *testing.T) {
	tests := []struct {
		name     string
		language string
		input    string
		want     string
	}{
		{
			name:     "zh to en",
			language: "en",
			input:    "首页",
			want:     "Home",
		},
		{
			name:     "en to zh",
			language: "zh",
			input:    "About Me",
			want:     "关于我",
		},
		{
			name:     "unknown stays",
			language: "en",
			input:    "Custom Title",
			want:     "Custom Title",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := localizeFixedTitle(tc.language, tc.input)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

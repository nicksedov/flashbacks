package config

import "testing"

func TestLoadPort(t *testing.T) {
	tests := []struct {
		name    string
		ocrPort string
		port    string
		want    string
	}{
		{
			name: "defaults to 5174 when nothing is set",
			want: "5174",
		},
		{
			name:    "reads OCR_PORT",
			ocrPort: "6001",
			want:    "6001",
		},
		{
			name:    "OCR_PORT takes precedence over PORT",
			ocrPort: "6001",
			port:    "6002",
			want:    "6001",
		},
		{
			name: "falls back to PORT when OCR_PORT is empty",
			port: "6002",
			want: "6002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OCR_PORT", tt.ocrPort)
			t.Setenv("PORT", tt.port)

			got := Load().Port
			if got != tt.want {
				t.Fatalf("Load().Port = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadHost(t *testing.T) {
	tests := []struct {
		name    string
		ocrHost string
		want    string
	}{
		{
			name: "defaults to 0.0.0.0 when nothing is set",
			want: "0.0.0.0",
		},
		{
			name:    "reads OCR_HOST",
			ocrHost: "127.0.0.1",
			want:    "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OCR_HOST", tt.ocrHost)

			got := Load().Host
			if got != tt.want {
				t.Fatalf("Load().Host = %q, want %q", got, tt.want)
			}
		})
	}
}

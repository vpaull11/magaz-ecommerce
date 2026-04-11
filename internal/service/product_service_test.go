package service

import (
	"mime/multipart"
	"testing"
)

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"normal", "photo.jpg", "photo.jpg"},
		{"path traversal", "../../etc/passwd", "passwd"},
		{"backslash", `C:\Users\evil.exe`, "evil.exe"},
		{"special chars", `file*name?.jpg`, "file_name_.jpg"},
		{"unicode", "фото.png", "фото.png"},
		{"pipe", "file|name.jpg", "file_name.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{Filename: tt.filename}
			got := SafeFilename(header)
			if got != tt.want {
				t.Errorf("SafeFilename(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestAllowedImageExt(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"photo.jpg", true},
		{"image.jpeg", true},
		{"image.png", true},
		{"image.webp", true},
		{"image.gif", true},
		{"image.bmp", false},
		{"document.pdf", false},
		{"script.php", false},
		{"image.JPG", true}, // case insensitive
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := AllowedImageExt(tt.filename)
			if got != tt.want {
				t.Errorf("AllowedImageExt(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestAllowedMIMEType(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/jpeg", true},
		{"image/png", true},
		{"image/webp", true},
		{"image/gif", true},
		{"image/bmp", false},
		{"text/html", false},
		{"application/pdf", false},
		{"application/octet-stream", false},
	}
	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := AllowedMIMEType(tt.mime)
			if got != tt.want {
				t.Errorf("AllowedMIMEType(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

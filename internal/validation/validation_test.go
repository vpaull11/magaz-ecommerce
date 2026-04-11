package validation

import (
	"testing"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "Иван Петров", false},
		{"valid name latin", "John Doe", false},
		{"valid name hyphen", "Анна-Мария", false},
		{"valid name apostrophe", "O'Brien", false},
		{"too short", "A", true},
		{"too long", string(make([]byte, 201)), true},
		{"is email", "test@example.com", true},
		{"has digits", "John123", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"valid email with plus", "user+tag@example.com", false},
		{"valid email subdomain", "user@sub.example.com", false},
		{"empty", "", true},
		{"no at sign", "userexample.com", true},
		{"no domain", "user@", true},
		{"no tld", "user@example", true},
		{"double at", "user@@example.com", true},
		{"spaces", "user @example.com", true},
		{"too long", "a" + string(make([]byte, 256)) + "@example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateZip(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid 6 digits", "123456", false},
		{"valid 5 digits", "12345", false},
		{"too short", "1234", true},
		{"with letters", "12345a", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateZip(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateZip(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", "Москва", false},
		{"valid with hyphen", "Санкт-Петербург", false},
		{"too short", "А", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCity(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

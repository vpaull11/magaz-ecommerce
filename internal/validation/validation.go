package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ─── Compiled patterns ────────────────────────────────────────────────────────

var (
	// emailPattern detects email-like strings (name@domain.tld)
	emailPattern = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	// namePattern: Unicode letters, spaces, hyphens, apostrophes — no digits/specials
	namePattern = regexp.MustCompile(`^[\p{L}\s'\-]+$`)
	// zipPattern: 5–10 digits only
	zipPattern = regexp.MustCompile(`^\d{5,10}$`)
	// streetPattern: letters, digits, spaces, . , / - (for house numbers like 12/1a)
	streetPattern = regexp.MustCompile(`^[\p{L}\p{N}\s\.,/\-]+$`)
)

// ─── Name validation ──────────────────────────────────────────────────────────

// ValidateName checks that the display name is a real name and not an email.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)

	if len(name) < 2 {
		return errors.New("имя должно содержать минимум 2 символа")
	}
	if len([]rune(name)) > 100 {
		return errors.New("имя слишком длинное (максимум 100 символов)")
	}

	// Reject email addresses in the name field
	if emailPattern.MatchString(name) {
		return errors.New("имя не должно быть email-адресом — введите своё настоящее имя")
	}

	// Must contain at least one Unicode letter
	hasLetter := false
	for _, r := range name {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return errors.New("имя должно содержать хотя бы одну букву")
	}

	// Reject digits in name
	for _, r := range name {
		if unicode.IsDigit(r) {
			return errors.New("имя не должно содержать цифры")
		}
	}

	// Allowed characters only
	if !namePattern.MatchString(name) {
		return fmt.Errorf("имя содержит недопустимые символы (допустимы: буквы, пробел, дефис)")
	}

	return nil
}

// ─── Address validation ───────────────────────────────────────────────────────

// ValidateLabel checks an address label (e.g. "Домашний", "Работа").
func ValidateLabel(label string) error {
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 50 {
		return errors.New("название адреса слишком длинное (максимум 50 символов)")
	}
	return nil
}

// ValidateCity checks a city name.
func ValidateCity(city string) error {
	city = strings.TrimSpace(city)
	if city == "" {
		return errors.New("укажите город")
	}
	if len([]rune(city)) < 2 {
		return errors.New("название города слишком короткое")
	}
	if len([]rune(city)) > 100 {
		return errors.New("название города слишком длинное (максимум 100 символов)")
	}
	if !namePattern.MatchString(city) {
		return errors.New("название города содержит недопустимые символы")
	}
	return nil
}

// ValidateStreet checks a street + house number.
func ValidateStreet(street string) error {
	street = strings.TrimSpace(street)
	if street == "" {
		return errors.New("укажите улицу и номер дома")
	}
	if len([]rune(street)) < 3 {
		return errors.New("адрес улицы слишком короткий")
	}
	if len([]rune(street)) > 200 {
		return errors.New("адрес улицы слишком длинный (максимум 200 символов)")
	}
	if !streetPattern.MatchString(street) {
		return errors.New("адрес улицы содержит недопустимые символы")
	}
	return nil
}

// ValidateZip checks a postal/zip code (digits only, 5–10 chars).
func ValidateZip(zip string) error {
	zip = strings.TrimSpace(zip)
	if zip == "" {
		return errors.New("укажите почтовый индекс")
	}
	if !zipPattern.MatchString(zip) {
		return errors.New("почтовый индекс должен содержать только цифры (5–10 знаков)")
	}
	return nil
}

// ValidateAddress validates all address fields and returns the first error found.
func ValidateAddress(label, city, street, zip string) error {
	if err := ValidateLabel(label); err != nil {
		return err
	}
	if err := ValidateCity(city); err != nil {
		return err
	}
	if err := ValidateStreet(street); err != nil {
		return err
	}
	return ValidateZip(zip)
}

// ─── Email validation ─────────────────────────────────────────────────────────

// ValidateEmail checks email format and length.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("укажите email")
	}
	if len(email) > 255 {
		return errors.New("email слишком длинный (максимум 255 символов)")
	}
	if !emailPattern.MatchString(email) {
		return errors.New("некорректный формат email")
	}
	return nil
}

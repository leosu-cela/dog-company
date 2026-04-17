package tool

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
)

var (
	ErrAccountInvalid  = errors.New("account must be a valid email or 3-30 chars of a-z, 0-9, _")
	ErrPasswordInvalid = errors.New("password must be 8-72 chars")

	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)
)

func NormalizeAccount(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func ValidateAccount(account string) error {
	if account == "" {
		return ErrAccountInvalid
	}
	if strings.Contains(account, "@") {
		if _, err := mail.ParseAddress(account); err != nil {
			return ErrAccountInvalid
		}
		return nil
	}
	if !usernamePattern.MatchString(account) {
		return ErrAccountInvalid
	}
	return nil
}

func ValidatePassword(pw string) error {
	if len(pw) < 8 || len(pw) > 72 {
		return ErrPasswordInvalid
	}
	return nil
}

package auth

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
)

// Anchored patterns: Go's regexp has no multiline mode by default, so ^...$
// match the whole string, and inputs are additionally length-capped before
// matching so a newline or oversized payload can't slip through.
var (
	usernameRe = regexp.MustCompile(`^\d{8}$`)
	passwordRe = regexp.MustCompile(`^[A-Z]{6,64}$`)
	matricRe   = regexp.MustCompile(`^[A-Z]{2,4}/\d{2}/\d{4}$`)
	pinRe      = regexp.MustCompile(`^\d{4}$`)
)

func ValidUsername(s string) bool { return len(s) == 8 && usernameRe.MatchString(s) }

// ValidPassword enforces the spec's staff password policy: uppercase letters
// only, at least 6. (ERROR_AUDIT.md discusses why this policy is weak.)
func ValidPassword(s string) bool { return len(s) <= 64 && passwordRe.MatchString(s) }

// NormalizeMatric trims and upper-cases a matric number, returning "" if it
// does not match the DEPT/YY/NNNN format.
func NormalizeMatric(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) > 20 || !matricRe.MatchString(s) {
		return ""
	}
	return s
}

func ValidPIN(s string) bool { return pinRe.MatchString(s) }

// NewPIN returns a uniformly random 4-digit PIN from crypto/rand.
func NewPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	s := n.String()
	return strings.Repeat("0", 4-len(s)) + s
}

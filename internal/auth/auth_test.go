package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"examshield/internal/models"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestStaffTokenRoundTrip(t *testing.T) {
	tk := NewTokens(secret, 15*time.Minute)
	raw, exp, err := tk.IssueStaff(&models.User{ID: 7, Role: models.RoleAdmin})
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(15*time.Minute), exp, 2*time.Second)
	c, err := tk.ParseStaff(raw)
	require.NoError(t, err)
	assert.Equal(t, "7", c.Subject)
	assert.Equal(t, models.RoleAdmin, c.Role)
}

func TestRejectsAlgNone(t *testing.T) {
	tk := NewTokens(secret, time.Minute)
	claims := StaffClaims{Role: models.RoleAdmin, Typ: "access", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "exam-shield", Subject: "1", Audience: jwt.ClaimStrings{AudStaff},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = tk.ParseStaff(raw)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestRejectsOtherHMACAlgAndWrongSecret(t *testing.T) {
	tk := NewTokens(secret, time.Minute)
	claims := StaffClaims{Role: models.RoleAdmin, Typ: "access", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "exam-shield", Subject: "1", Audience: jwt.ClaimStrings{AudStaff},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	raw, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(secret)
	_, err := tk.ParseStaff(raw)
	assert.Error(t, err, "HS512 must be rejected: algorithm is pinned")

	other := NewTokens([]byte("ffffffffffffffffffffffffffffffff"), time.Minute)
	raw2, _, _ := other.IssueStaff(&models.User{ID: 1, Role: models.RoleAdmin})
	_, err = tk.ParseStaff(raw2)
	assert.Error(t, err)
}

func TestTokenKindsAreNotInterchangeable(t *testing.T) {
	tk := NewTokens(secret, time.Minute)
	student, err := tk.IssueStudent(1, 2, 3, strings.Repeat("a", 64), time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = tk.ParseStaff(student)
	assert.Error(t, err, "student token must not authenticate staff")

	staff, _, _ := tk.IssueStaff(&models.User{ID: 1, Role: models.RoleProctor})
	_, err = tk.ParseStudent(staff)
	assert.Error(t, err)

	sc, err := tk.ParseStudent(student)
	require.NoError(t, err)
	assert.EqualValues(t, 1, sc.SessionID())
}

func TestExpiredTokenRejected(t *testing.T) {
	tk := NewTokens(secret, time.Minute)
	raw, err := tk.IssueStudent(1, 2, 3, "fp", time.Now().Add(-time.Minute))
	require.NoError(t, err)
	_, err = tk.ParseStudent(raw)
	assert.Error(t, err)
}

func TestValidators(t *testing.T) {
	assert.True(t, ValidUsername("19990045"))
	for _, bad := range []string{"1999004", "199900451", "1999004a", "19990045\n", "１９９９００４５"} {
		assert.False(t, ValidUsername(bad), "%q", bad)
	}
	assert.True(t, ValidPassword("OLAWALE"))
	for _, bad := range []string{"OLAWA", "olawale", "OLAWALE1", "OLAWALE\n", ""} {
		assert.False(t, ValidPassword(bad), "%q", bad)
	}
	assert.Equal(t, "IFT/22/9203", NormalizeMatric(" ift/22/9203 "))
	for _, bad := range []string{"IFT/22/920", "IFT-22-9203", "IFT/22/9203\nX", "I/22/9203", "' OR 1=1 --"} {
		assert.Equal(t, "", NormalizeMatric(bad), "%q", bad)
	}
	for i := 0; i < 50; i++ {
		assert.True(t, ValidPIN(NewPIN()))
	}
}

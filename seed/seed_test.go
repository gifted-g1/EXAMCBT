package seed

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"examshield/internal/models"
	"examshield/internal/store"
)

func TestRunIsIdempotent(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	for i := 0; i < 3; i++ {
		require.NoError(t, Run(db, Options{BcryptCost: 4, DemoExam: true}))
	}
	var users, students, exams, questions int64
	db.Model(&models.User{}).Count(&users)
	db.Model(&models.Student{}).Count(&students)
	db.Model(&models.Exam{}).Count(&exams)
	db.Model(&models.Question{}).Count(&questions)
	assert.EqualValues(t, 8, users)
	assert.EqualValues(t, 12, students)
	assert.EqualValues(t, 1, exams)
	assert.EqualValues(t, 5, questions)

	var u models.User
	require.NoError(t, db.Where("username = ?", "20010078").First(&u).Error)
	assert.Equal(t, models.RoleAdmin, u.Role)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("MUSTAPHA")))
}

func TestWriteSQL(t *testing.T) {
	var b strings.Builder
	require.NoError(t, WriteSQL(&b, 4))
	sql := b.String()
	assert.Equal(t, 2, strings.Count(sql, "INSERT IGNORE INTO"))
	assert.Equal(t, len(Staff), strings.Count(sql, "$2a$04$"))
	assert.NotContains(t, sql, "OLAWALE'", "plaintext passwords must not appear")
}

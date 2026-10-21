package repositories

import (
	"context"
	"time"

	"examshield/internal/models"
)

func (r *Repo) UserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	if err := r.q(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *Repo) UserByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	if err := r.q(ctx).First(&u, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &u, nil
}

func (r *Repo) ListUsers(ctx context.Context) ([]models.User, error) {
	var out []models.User
	err := r.q(ctx).Order("full_name").Find(&out).Error
	return out, err
}

func (r *Repo) CreateUser(ctx context.Context, u *models.User) error {
	return r.q(ctx).Create(u).Error
}

func (r *Repo) UpdateUser(ctx context.Context, id int64, fields map[string]any) error {
	res := r.q(ctx).Model(&models.User{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) CountActiveAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := r.q(ctx).Model(&models.User{}).Where("role = ? AND is_active = ?", models.RoleAdmin, true).Count(&n).Error
	return n, err
}

// --- Refresh tokens -----------------------------------------------------------

func (r *Repo) CreateRefresh(ctx context.Context, t *models.RefreshToken) error {
	return r.q(ctx).Create(t).Error
}

// RefreshByHashForUpdate loads a refresh token row locked for rotation.
func (r *Repo) RefreshByHashForUpdate(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.forUpdate(r.q(ctx)).Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, notFound(err)
	}
	return &t, nil
}

func (r *Repo) RevokeRefresh(ctx context.Context, id int64) error {
	return r.q(ctx).Model(&models.RefreshToken{}).Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", time.Now().UTC()).Error
}

func (r *Repo) RevokeFamily(ctx context.Context, family string) error {
	return r.q(ctx).Model(&models.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", family).
		Update("revoked_at", time.Now().UTC()).Error
}

func (r *Repo) RevokeUserTokens(ctx context.Context, userID int64) error {
	return r.q(ctx).Model(&models.RefreshToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", time.Now().UTC()).Error
}

func (r *Repo) PurgeExpiredRefresh(ctx context.Context, before time.Time) (int64, error) {
	res := r.q(ctx).Where("expires_at < ?", before).Delete(&models.RefreshToken{})
	return res.RowsAffected, res.Error
}

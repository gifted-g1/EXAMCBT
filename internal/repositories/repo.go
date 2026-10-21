// Package repositories is the only package that builds SQL. Every query uses
// GORM parameter binding; no user input is ever concatenated into SQL.
package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"examshield/internal/models"
)

var ErrNotFound = errors.New("not found")

type Repo struct{ db *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// DB exposes the handle for health checks.
func (r *Repo) DB() *gorm.DB { return r.db }

// Tx runs fn inside a transaction with a Repo bound to it.
func (r *Repo) Tx(ctx context.Context, fn func(tx *Repo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&Repo{db: tx})
	})
}

func (r *Repo) q(ctx context.Context) *gorm.DB { return r.db.WithContext(ctx) }

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// forUpdate adds SELECT ... FOR UPDATE on dialects that support it.
func (r *Repo) forUpdate(q *gorm.DB) *gorm.DB {
	if r.db.Dialector.Name() == "sqlite" {
		return q // SQLite serialises writers; one connection in standalone mode.
	}
	return q.Clauses(clause.Locking{Strength: "UPDATE"})
}

// --- Audit -------------------------------------------------------------------

func (r *Repo) Audit(ctx context.Context, actorID *int64, action, target, ip string, details map[string]any) error {
	return r.q(ctx).Create(&models.AuditLog{ActorID: actorID, Action: action, Target: target, ClientIP: ip, Details: details}).Error
}

func (r *Repo) ListAudit(ctx context.Context, limit int) ([]models.AuditLog, error) {
	var out []models.AuditLog
	err := r.q(ctx).Order("id DESC").Limit(limit).Find(&out).Error
	return out, err
}

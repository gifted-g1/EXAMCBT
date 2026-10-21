package repositories

import (
	"context"
	"strings"

	"gorm.io/gorm/clause"

	"examshield/internal/models"
)

func (r *Repo) StudentByMatric(ctx context.Context, matric string) (*models.Student, error) {
	var s models.Student
	if err := r.q(ctx).Where("matric_no = ?", matric).First(&s).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

func (r *Repo) StudentByID(ctx context.Context, id int64) (*models.Student, error) {
	var s models.Student
	if err := r.q(ctx).First(&s, id).Error; err != nil {
		return nil, notFound(err)
	}
	return &s, nil
}

// ListStudents returns students whose matric or name contains search.
func (r *Repo) ListStudents(ctx context.Context, search string, limit, offset int) ([]models.Student, int64, error) {
	q := r.q(ctx).Model(&models.Student{})
	if search = strings.TrimSpace(search); search != "" {
		like := "%" + escapeLike(search) + "%"
		q = q.Where("matric_no LIKE ? ESCAPE '!' OR full_name LIKE ? ESCAPE '!'", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []models.Student
	err := q.Order("matric_no").Limit(limit).Offset(offset).Find(&out).Error
	return out, total, err
}

// UpsertStudents inserts roster rows and updates matric numbers that already
// exist. Only fields a row actually carries are overwritten: a CSV without a
// department or photo column never blanks existing values.
func (r *Repo) UpsertStudents(ctx context.Context, rows []models.Student) error {
	if len(rows) == 0 {
		return nil
	}
	groups := map[[2]bool][]models.Student{}
	for _, s := range rows {
		k := [2]bool{s.PhotoURL != "", s.Department != ""}
		groups[k] = append(groups[k], s)
	}
	return r.Tx(ctx, func(tx *Repo) error {
		for k, g := range groups {
			cols := []string{"full_name", "updated_at"}
			if k[0] {
				cols = append(cols, "photo_url")
			}
			if k[1] {
				cols = append(cols, "department")
			}
			if err := tx.q(ctx).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "matric_no"}},
				DoUpdates: clause.AssignmentColumns(cols),
			}).CreateInBatches(g, 200).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repo) CreateStudent(ctx context.Context, s *models.Student) error {
	return r.q(ctx).Create(s).Error
}

func (r *Repo) SetStudentPhoto(ctx context.Context, id int64, url string) error {
	res := r.q(ctx).Model(&models.Student{}).Where("id = ?", id).Update("photo_url", url)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

// DeleteStudent removes the student; sessions, answers, violations and AI
// findings cascade via foreign keys.
func (r *Repo) DeleteStudent(ctx context.Context, id int64) error {
	res := r.q(ctx).Delete(&models.Student{}, id)
	if res.Error == nil && res.RowsAffected == 0 {
		return ErrNotFound
	}
	return res.Error
}

func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

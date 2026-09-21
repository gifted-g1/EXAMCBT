package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role string

const (
	RoleSuperAdmin Role = "SUPER_ADMIN"
	RoleAdmin      Role = "ADMIN"
	RoleLecturer   Role = "LECTURER"
	RoleStudent    Role = "STUDENT"
)

// Organization represents an institution/school/examination body.
type Organization struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Code      string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"code"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User is the base identity record for every actor in the system,
// regardless of role. Role-specific data lives in Student/Lecturer/
// Administrator tables, linked 1:1 by UserID.
type User struct {
	ID             uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:char(36);index;not null" json:"organization_id"`
	Organization   Organization   `json:"-"`
	Email          string         `gorm:"type:varchar(191);uniqueIndex;not null" json:"email"`
	PasswordHash   string         `gorm:"type:varchar(255);not null" json:"-"`
	FullName       string         `gorm:"type:varchar(255);not null" json:"full_name"`
	Role           Role           `gorm:"type:varchar(20);not null;index" json:"role"`
	Active         bool           `gorm:"default:true" json:"active"`
	Suspended      bool           `gorm:"default:false" json:"suspended"`
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (o *Organization) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// Student holds role-specific attributes for a user with RoleStudent.
type Student struct {
	ID              uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	UserID          uuid.UUID  `gorm:"type:char(36);uniqueIndex;not null" json:"user_id"`
	User            User       `json:"user"`
	MatricNumber    string     `gorm:"type:varchar(100);uniqueIndex" json:"matric_number"`
	Department      string     `gorm:"type:varchar(150)" json:"department"`
	ReferenceFaceID *uuid.UUID `gorm:"type:char(36)" json:"reference_face_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (s *Student) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// Lecturer holds role-specific attributes for a user with RoleLecturer.
type Lecturer struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:char(36);uniqueIndex;not null" json:"user_id"`
	User        User      `json:"user"`
	StaffNumber string    `gorm:"type:varchar(100)" json:"staff_number"`
	Department  string    `gorm:"type:varchar(150)" json:"department"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (l *Lecturer) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// Administrator holds role-specific attributes for a user with RoleAdmin.
type Administrator struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:char(36);uniqueIndex;not null" json:"user_id"`
	User      User      `json:"user"`
	Title     string    `gorm:"type:varchar(100)" json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *Administrator) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// RefreshToken tracks issued refresh tokens so they can be revoked
// (logout, suspension, rotation) independently of JWT expiry.
type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:char(36);index;not null" json:"user_id"`
	TokenHash string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

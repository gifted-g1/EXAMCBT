package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ExamMode string

const (
	ExamModeOnline ExamMode = "ONLINE"
	ExamModeLAN    ExamMode = "LAN"
)

type ExamStatus string

const (
	ExamStatusDraft     ExamStatus = "DRAFT"
	ExamStatusScheduled ExamStatus = "SCHEDULED"
	ExamStatusPublished ExamStatus = "PUBLISHED"
	ExamStatusActive    ExamStatus = "ACTIVE"
	ExamStatusEnded     ExamStatus = "ENDED"
	ExamStatusArchived  ExamStatus = "ARCHIVED"
)

type QuestionType string

const (
	QuestionMultipleChoice QuestionType = "MULTIPLE_CHOICE"
	QuestionTrueFalse      QuestionType = "TRUE_FALSE"
	QuestionShortAnswer    QuestionType = "SHORT_ANSWER"
)

// Exam is the top-level examination definition created by a lecturer
// and reviewed/scheduled/published by an admin.
type Exam struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID        uuid.UUID      `gorm:"type:uuid;index;not null" json:"organization_id"`
	CreatedByUserID       uuid.UUID      `gorm:"type:uuid;index;not null" json:"created_by_user_id"`
	Title                 string         `gorm:"not null" json:"title"`
	Course                string         `json:"course"`
	CourseCode            string         `json:"course_code"`
	DurationMinutes       int            `gorm:"not null" json:"duration_minutes"`
	StartAt               time.Time      `json:"start_at"`
	EndAt                 time.Time      `json:"end_at"`
	Mode                  ExamMode       `gorm:"not null;default:'ONLINE'" json:"mode"`
	Status                ExamStatus     `gorm:"not null;default:'DRAFT';index" json:"status"`
	RandomizeQuestions    bool           `gorm:"default:false" json:"randomize_questions"`
	RandomizeAnswers      bool           `gorm:"default:false" json:"randomize_answers"`
	AllowNavigation       bool           `gorm:"default:true" json:"allow_navigation"`
	EnableAIMonitoring    bool           `gorm:"default:true" json:"enable_ai_monitoring"`
	RequireFaceVerification bool         `gorm:"default:true" json:"require_face_verification"`
	PassingScore          float64        `json:"passing_score"`
	Instructions          string         `json:"instructions"`
	// LAN-mode runtime info, populated when the exam server is started.
	LANServerIP           string         `json:"lan_server_ip,omitempty"`
	LANServerPort         int            `json:"lan_server_port,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`

	Questions []Question `json:"questions,omitempty"`
}

func (e *Exam) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// Question belongs to an exam. Additional question types can be added
// by extending QuestionType and the grading logic in services/exam.
type Question struct {
	ID          uuid.UUID    `gorm:"type:uuid;primaryKey" json:"id"`
	ExamID      uuid.UUID    `gorm:"type:uuid;index;not null" json:"exam_id"`
	Type        QuestionType `gorm:"not null" json:"type"`
	Text        string       `gorm:"not null" json:"text"`
	Points      float64      `gorm:"default:1" json:"points"`
	OrderIndex  int          `json:"order_index"`
	CorrectText string       `json:"correct_text,omitempty"` // used for SHORT_ANSWER / TRUE_FALSE
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	Options []Option `json:"options,omitempty"`
}

func (q *Question) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

// Option is a possible answer for a MULTIPLE_CHOICE question.
type Option struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	QuestionID uuid.UUID `gorm:"type:uuid;index;not null" json:"question_id"`
	Text       string    `gorm:"not null" json:"text"`
	IsCorrect  bool      `gorm:"default:false" json:"is_correct"`
	OrderIndex int       `json:"order_index"`
}

func (o *Option) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

type AttemptStatus string

const (
	AttemptNotStarted AttemptStatus = "NOT_STARTED"
	AttemptInProgress AttemptStatus = "IN_PROGRESS"
	AttemptSubmitted  AttemptStatus = "SUBMITTED"
	AttemptDisconnected AttemptStatus = "DISCONNECTED"
)

// ExamAttempt tracks one student's single attempt at an exam. A unique
// index on (exam_id, student_id) enforces one attempt per student,
// preventing duplicate submissions at the database level.
type ExamAttempt struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	ExamID         uuid.UUID     `gorm:"type:uuid;index:idx_exam_student,unique;not null" json:"exam_id"`
	StudentID      uuid.UUID     `gorm:"type:uuid;index:idx_exam_student,unique;not null" json:"student_id"`
	Status         AttemptStatus `gorm:"not null;default:'NOT_STARTED';index" json:"status"`
	VerifiedFace   bool          `gorm:"default:false" json:"verified_face"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	SubmittedAt    *time.Time    `json:"submitted_at,omitempty"`
	Score          *float64      `json:"score,omitempty"`
	IPAddress      string        `json:"ip_address,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (a *ExamAttempt) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Answer records a single student response to a single question within
// an attempt. A unique index prevents double-answering the same
// question within an attempt (updates overwrite instead).
type Answer struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	AttemptID    uuid.UUID `gorm:"type:uuid;index:idx_attempt_question,unique;not null" json:"attempt_id"`
	QuestionID   uuid.UUID `gorm:"type:uuid;index:idx_attempt_question,unique;not null" json:"question_id"`
	SelectedOptionID *uuid.UUID `gorm:"type:uuid" json:"selected_option_id,omitempty"`
	TextAnswer   string    `json:"text_answer,omitempty"`
	IsCorrect    *bool     `json:"is_correct,omitempty"`
	AwardedPoints float64  `json:"awarded_points"`
	AnsweredAt   time.Time `json:"answered_at"`
}

func (a *Answer) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

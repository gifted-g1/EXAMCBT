// Package models holds the GORM entities. Column definitions mirror
// migrations/000001_init.up.sql, which is authoritative for MySQL; the GORM
// tags exist so standalone (SQLite) mode and tests can AutoMigrate the same
// shape.
package models

import (
	"regexp"
	"time"

	"gorm.io/datatypes"
)

type Role string

const (
	RoleProctor Role = "proctor"
	RoleAdmin   Role = "admin"
)

func (r Role) Valid() bool { return r == RoleProctor || r == RoleAdmin }

type User struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"size:20;not null;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"size:72;not null" json:"-"`
	FullName     string    `gorm:"size:120;not null" json:"full_name"`
	Role         Role      `gorm:"size:16;not null;index:idx_role" json:"role"`
	IsActive     bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Student struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MatricNo   string    `gorm:"size:20;not null;uniqueIndex" json:"matric_no"`
	FullName   string    `gorm:"size:120;not null" json:"full_name"`
	Department string    `gorm:"size:120" json:"department,omitempty"`
	PhotoURL   string    `gorm:"size:255" json:"photo_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ExamStatus string

const (
	ExamDraft     ExamStatus = "DRAFT"
	ExamScheduled ExamStatus = "SCHEDULED"
	ExamActive    ExamStatus = "ACTIVE"
	ExamClosed    ExamStatus = "CLOSED"
)

type Exam struct {
	ID          int64              `gorm:"primaryKey;autoIncrement" json:"id"`
	Title       string             `gorm:"size:150;not null" json:"title"`
	Description string             `gorm:"type:text" json:"description"`
	CreatedBy   int64              `gorm:"not null;index" json:"created_by"`
	StartsAt    time.Time          `gorm:"not null;index:idx_status_window,priority:2" json:"starts_at"`
	EndsAt      time.Time          `gorm:"not null;index:idx_status_window,priority:3" json:"ends_at"`
	Status      ExamStatus         `gorm:"size:16;not null;default:DRAFT;index:idx_status_window,priority:1" json:"status"`
	LANCIDR     string             `gorm:"column:lan_cidr;size:43;not null" json:"lan_cidr"`
	PinCode     *string            `gorm:"size:4" json:"pin_code,omitempty"`
	Proctoring  ProctoringSettings `gorm:"column:proctoring;type:json;serializer:json" json:"proctoring"`
	// DurationMinutes is each student's time limit from when they start;
	// 0 means "until the exam window ends". Never runs past EndsAt.
	DurationMinutes int `gorm:"not null;default:0" json:"duration_minutes"`
	// QuestionsPerStudent, when > 0 and smaller than the pool, gives each
	// student their own random draw of that many questions.
	QuestionsPerStudent int        `gorm:"not null;default:0" json:"questions_per_student"`
	MediaPurgedAt       *time.Time `json:"media_purged_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Creator             *User      `gorm:"foreignKey:CreatedBy;constraint:OnDelete:RESTRICT" json:"-"`
	QuestionsCount      int64      `gorm:"-" json:"questions_count"`
}

// Deadline is when a session started at startedAt must be submitted.
func (e *Exam) Deadline(startedAt time.Time) time.Time {
	if e.DurationMinutes > 0 {
		if d := startedAt.Add(time.Duration(e.DurationMinutes) * time.Minute); d.Before(e.EndsAt) {
			return d
		}
	}
	return e.EndsAt
}

// BlankRe finds the blanks ("_____", three or more underscores) in a GAP prompt.
var BlankRe = regexp.MustCompile(`_{3,}`)

// CountBlanks returns how many blanks a prompt has.
func CountBlanks(prompt string) int { return len(BlankRe.FindAllStringIndex(prompt, -1)) }

// PinRequired reports whether students must present a PIN.
func (e *Exam) PinRequired() bool { return e.PinCode != nil && *e.PinCode != "" }

type QuestionKind string

const (
	QuestionMCQ  QuestionKind = "MCQ"
	QuestionText QuestionKind = "TEXT"
	QuestionGap  QuestionKind = "GAP" // fill in the gap(s), auto-marked
)

type Question struct {
	ID            int64                       `gorm:"primaryKey;autoIncrement" json:"id"`
	ExamID        int64                       `gorm:"not null;index:idx_exam_position,priority:1" json:"exam_id"`
	Position      int                         `gorm:"not null;default:0;index:idx_exam_position,priority:2" json:"position"`
	Kind          QuestionKind                `gorm:"size:8;not null;default:MCQ" json:"kind"`
	Prompt        string                      `gorm:"type:text;not null" json:"prompt"`
	Options       datatypes.JSONSlice[string] `gorm:"type:json" json:"options"`
	CorrectOption *int                        `json:"correct_option,omitempty"`
	// GAP questions: accepted answers for each blank, in order. Staff only.
	Blanks datatypes.JSONSlice[[]string] `gorm:"type:json" json:"blanks,omitempty"`
	Marks  float64                       `gorm:"type:decimal(5,2);not null;default:1" json:"marks"`
	// Written (TEXT) questions only: the examiner's key points and an
	// optional model answer. Never sent to students.
	MarkingGuide datatypes.JSONSlice[MarkingPoint] `gorm:"type:json" json:"marking_guide,omitempty"`
	ModelAnswer  string                            `gorm:"type:text" json:"model_answer,omitempty"`
	Exam         *Exam                             `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

// MarkingPoint is one key point an examiner expects in a written answer,
// worth Marks, recognised by any of its Keywords.
type MarkingPoint struct {
	Point    string   `json:"point"`
	Marks    float64  `json:"marks"`
	Keywords []string `json:"keywords"`
}

type SessionStatus string

const (
	SessionInit      SessionStatus = "INIT"
	SessionActive    SessionStatus = "ACTIVE"
	SessionSubmitted SessionStatus = "SUBMITTED"
	SessionClosed    SessionStatus = "CLOSED"
)

type ExamSession struct {
	ID             int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	ExamID         int64         `gorm:"not null;uniqueIndex:uq_exam_student,priority:1" json:"exam_id"`
	StudentID      int64         `gorm:"not null;uniqueIndex:uq_exam_student,priority:2;index" json:"student_id"`
	DeviceFP       string        `gorm:"column:device_fp;size:64;not null" json:"device_fp"`
	ClientIP       string        `gorm:"size:45;not null" json:"client_ip"`
	StartedAt      time.Time     `gorm:"not null" json:"started_at"`
	EndedAt        *time.Time    `json:"ended_at,omitempty"`
	LastSeenAt     *time.Time    `json:"last_seen_at,omitempty"`
	IntegrityScore float64       `gorm:"type:decimal(5,2);not null;default:100" json:"integrity_score"`
	FaceMatchScore *float64      `gorm:"type:decimal(4,3)" json:"face_match_score,omitempty"`
	ReferenceKey   string        `gorm:"size:255" json:"-"`
	IDPhotoKey     string        `gorm:"column:id_photo_key;size:255" json:"-"`
	ConsentAt      *time.Time    `json:"consent_at,omitempty"`
	ConsentVersion string        `gorm:"size:16" json:"consent_version,omitempty"`
	Score          *float64      `gorm:"type:decimal(7,2)" json:"score,omitempty"`
	AutoSubmitted  bool          `gorm:"not null;default:false" json:"auto_submitted"`
	Status         SessionStatus `gorm:"size:16;not null;default:INIT;index:idx_status" json:"status"`
	// QuestionIDs are this student's questions in their order; nil means
	// every question of the exam in the standard order.
	QuestionIDs datatypes.JSONSlice[int64] `gorm:"type:json" json:"question_ids,omitempty"`
	Exam        *Exam                      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Student     *Student                   `gorm:"constraint:OnDelete:CASCADE" json:"student,omitempty"`
}

type Severity string

const (
	SevLow      Severity = "LOW"
	SevMedium   Severity = "MEDIUM"
	SevHigh     Severity = "HIGH"
	SevCritical Severity = "CRITICAL"
)

func (s Severity) Valid() bool {
	switch s {
	case SevLow, SevMedium, SevHigh, SevCritical:
		return true
	}
	return false
}

type ViolationSource string

const (
	SourceClient  ViolationSource = "CLIENT"
	SourceAI      ViolationSource = "AI"
	SourceProctor ViolationSource = "PROCTOR"
)

type ReviewStatus string

const (
	ReviewPending   ReviewStatus = "PENDING"
	ReviewConfirmed ReviewStatus = "CONFIRMED"
	ReviewDismissed ReviewStatus = "DISMISSED"
)

type Violation struct {
	ID           int64             `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID    int64             `gorm:"not null;index:idx_session_time,priority:1" json:"session_id"`
	EventType    string            `gorm:"size:50;not null" json:"event_type"`
	Severity     Severity          `gorm:"size:10;not null" json:"severity"`
	Source       ViolationSource   `gorm:"size:10;not null;default:CLIENT" json:"source"`
	OccurredAt   time.Time         `gorm:"not null;index:idx_session_time,priority:2" json:"occurred_at"`
	MediaRef     *string           `gorm:"size:255" json:"media_ref,omitempty"`
	AIConfidence *float64          `gorm:"type:decimal(4,3)" json:"ai_confidence,omitempty"`
	Metadata     datatypes.JSONMap `gorm:"type:json" json:"metadata,omitempty"`
	ReviewStatus ReviewStatus      `gorm:"size:10;not null;default:PENDING" json:"review_status"`
	ReviewedBy   *int64            `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time        `json:"reviewed_at,omitempty"`
	Session      *ExamSession      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

type AIFinding struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID int64          `gorm:"not null;index:idx_session_model,priority:1" json:"session_id"`
	ModelName string         `gorm:"size:50;not null;index:idx_session_model,priority:2" json:"model_name"`
	Inference datatypes.JSON `gorm:"type:json;not null" json:"inference"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	Session   *ExamSession   `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

type Answer struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID  int64     `gorm:"not null;uniqueIndex:uq_session_q,priority:1" json:"session_id"`
	QuestionID int64     `gorm:"not null;uniqueIndex:uq_session_q,priority:2;index" json:"question_id"`
	Response   string    `gorm:"type:text" json:"response"`
	SavedAt    time.Time `gorm:"not null" json:"saved_at"`
	// Examiner marking of written answers. AwardedMarks stays nil until a
	// human confirms a mark; Suggestion holds the last AI suggestion.
	AwardedMarks  *float64          `gorm:"type:decimal(5,2)" json:"awarded_marks,omitempty"`
	MarkedBy      *int64            `json:"marked_by,omitempty"`
	MarkedAt      *time.Time        `json:"marked_at,omitempty"`
	MarkerComment string            `gorm:"type:text" json:"marker_comment,omitempty"`
	Suggestion    datatypes.JSONMap `gorm:"type:json" json:"suggestion,omitempty"`
	Session       *ExamSession      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Question      *Question         `gorm:"constraint:OnDelete:CASCADE" json:"-"`
}

// RefreshToken stores the SHA-256 of an opaque refresh token. Tokens in the
// same FamilyID descend from one login; presenting a revoked member of a
// family (token reuse) revokes the whole family.
type RefreshToken struct {
	ID        int64      `gorm:"primaryKey;autoIncrement"`
	UserID    int64      `gorm:"not null;index"`
	FamilyID  string     `gorm:"size:36;not null;index"`
	TokenHash string     `gorm:"size:64;not null;uniqueIndex"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time `gorm:""`
	CreatedAt time.Time
	User      *User `gorm:"constraint:OnDelete:CASCADE"`
}

type AuditLog struct {
	ID        int64             `gorm:"primaryKey;autoIncrement" json:"id"`
	ActorID   *int64            `gorm:"index" json:"actor_id,omitempty"`
	Action    string            `gorm:"size:64;not null" json:"action"`
	Target    string            `gorm:"size:64" json:"target"`
	Details   datatypes.JSONMap `gorm:"type:json" json:"details,omitempty"`
	ClientIP  string            `gorm:"size:45" json:"client_ip"`
	CreatedAt time.Time         `gorm:"index" json:"created_at"`
}

// All returns every model in dependency order, for AutoMigrate.
func All() []any {
	return []any{&User{}, &Student{}, &Exam{}, &Question{}, &ExamSession{},
		&Violation{}, &AIFinding{}, &Answer{}, &RefreshToken{}, &AuditLog{}}
}

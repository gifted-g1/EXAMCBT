package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"examshield/internal/models"
	"examshield/internal/repositories"
	"examshield/internal/websocket"
)

// MonitoringService is the sole bridge between Go and the Python AI
// service. React never talks to Python directly; every frame and
// verification request is validated and authorized here first.
type MonitoringService struct {
	repo       *repositories.MonitoringRepository
	hub        *websocket.Hub
	aiBaseURL  string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewMonitoringService(repo *repositories.MonitoringRepository, hub *websocket.Hub, aiBaseURL string, logger *slog.Logger) *MonitoringService {
	return &MonitoringService{
		repo:      repo,
		hub:       hub,
		aiBaseURL: aiBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

type verifyFaceAIRequest struct {
	StudentID     string `json:"student_id"`
	ImageBase64   string `json:"image_base64"`
	IsReference   bool   `json:"is_reference"`
}

type verifyFaceAIResponse struct {
	Matched    bool    `json:"matched"`
	Confidence float64 `json:"confidence"`
}

// VerifyFace forwards a captured frame to the AI service for
// comparison against the student's registered reference face (or
// registers it as the reference when isReference is true), then
// persists only the verification result — never the raw image.
func (s *MonitoringService) VerifyFace(ctx context.Context, examID *uuid.UUID, attemptID *uuid.UUID, studentID uuid.UUID, imageBase64 string, isReference bool) (*models.FaceVerification, error) {
	reqBody, _ := json.Marshal(verifyFaceAIRequest{
		StudentID:   studentID.String(),
		ImageBase64: imageBase64,
		IsReference: isReference,
	})

	resp, err := s.httpClient.Post(s.aiBaseURL+"/verify-face", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ai service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var aiResp verifyFaceAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return nil, fmt.Errorf("invalid ai service response: %w", err)
	}

	record := &models.FaceVerification{
		StudentID:   studentID,
		Matched:     aiResp.Matched,
		Confidence:  aiResp.Confidence,
		IsReference: isReference,
	}
	if examID != nil {
		record.ExamID = *examID
	}
	record.AttemptID = attemptID

	if err := s.repo.SaveFaceVerification(ctx, record); err != nil {
		return nil, err
	}

	if examID != nil {
		s.hub.Broadcast(*examID, websocket.EventFaceVerification, map[string]interface{}{
			"student_id": studentID.String(),
			"matched":    aiResp.Matched,
			"confidence": aiResp.Confidence,
		})
		if aiResp.Matched {
			s.hub.Broadcast(*examID, websocket.EventStudentVerified, map[string]string{"student_id": studentID.String()})
		}
	}

	return record, nil
}

type analyzeFrameAIRequest struct {
	StudentID   string `json:"student_id"`
	ImageBase64 string `json:"image_base64"`
}

type analyzeFrameAIResponse struct {
	Events []struct {
		EventType   string  `json:"event_type"`
		Severity    string  `json:"severity"`
		Confidence  float64 `json:"confidence"`
		Description string  `json:"description"`
	} `json:"events"`
}

// AnalyzeFrame forwards a monitoring frame to the AI service for
// behavior analysis and persists every returned event as an AIEvent.
// Severities are classified by the AI service and reviewed by humans —
// Go never auto-escalates an event into a cheating determination.
func (s *MonitoringService) AnalyzeFrame(ctx context.Context, examID, attemptID, studentID uuid.UUID, imageBase64 string) ([]models.AIEvent, error) {
	reqBody, _ := json.Marshal(analyzeFrameAIRequest{StudentID: studentID.String(), ImageBase64: imageBase64})

	resp, err := s.httpClient.Post(s.aiBaseURL+"/analyze-frame", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ai service unreachable: %w", err)
	}
	defer resp.Body.Close()

	var aiResp analyzeFrameAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return nil, fmt.Errorf("invalid ai service response: %w", err)
	}

	var saved []models.AIEvent
	for _, e := range aiResp.Events {
		event := models.AIEvent{
			AttemptID:   attemptID,
			StudentID:   studentID,
			ExamID:      examID,
			EventType:   e.EventType,
			Severity:    models.Severity(e.Severity),
			Confidence:  e.Confidence,
			Description: e.Description,
			Timestamp:   time.Now(),
		}
		if err := s.repo.SaveAIEvent(ctx, &event); err != nil {
			s.logger.Error("failed to persist AI event", "error", err)
			continue
		}
		saved = append(saved, event)

		s.hub.Broadcast(examID, websocket.EventAIMonitoring, event)
		if event.Severity == models.SeverityHigh || event.Severity == models.SeverityCritical {
			s.hub.Broadcast(examID, websocket.EventSuspiciousActivity, event)
		}
	}
	return saved, nil
}

func (s *MonitoringService) ListEventsForExam(ctx context.Context, examID uuid.UUID) ([]models.AIEvent, error) {
	return s.repo.ListAIEventsByExam(ctx, examID)
}

func (s *MonitoringService) ReviewEvent(ctx context.Context, eventID, reviewerID uuid.UUID) error {
	return s.repo.MarkAIEventReviewed(ctx, eventID, reviewerID)
}

func (s *MonitoringService) RecordCameraStatus(examID, studentID uuid.UUID, active bool) {
	s.hub.Broadcast(examID, websocket.EventCameraStatus, map[string]interface{}{
		"student_id": studentID.String(),
		"active":     active,
	})
}

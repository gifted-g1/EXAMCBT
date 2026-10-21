package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"path"
	"strings"

	"examshield/internal/ai"
	"examshield/internal/auth"
	"examshield/internal/models"
	"examshield/internal/repositories"
	"examshield/internal/store"
)

const maxRosterRows = 20000

type RosterService struct {
	repo  *repositories.Repo
	blobs store.Blobs
	log   *slog.Logger
}

func NewRosterService(repo *repositories.Repo, blobs store.Blobs, log *slog.Logger) *RosterService {
	return &RosterService{repo: repo, blobs: blobs, log: log}
}

// RowError reports one rejected CSV line.
type RowError struct {
	Line  int    `json:"line"`
	Error string `json:"error"`
}

type ImportResult struct {
	Imported int        `json:"imported"`
	Rejected []RowError `json:"rejected"`
}

// ImportCSV loads a roster with header matric_no,full_name[,department][,photo_url].
// Every row is validated first; duplicates within the file are rejected; if
// nothing is valid nothing is written. Existing matric numbers are updated.
// photo_url values are kept only when they reference a photo already
// uploaded to Exam Shield ("blob:roster/..."); external URLs are never
// fetched (SSRF / offline hall).
func (s *RosterService) ImportCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, invalid("CSV is empty or unreadable")
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	mi, ok1 := col["matric_no"]
	ni, ok2 := col["full_name"]
	pi, hasPhoto := col["photo_url"]
	di, hasDept := col["department"]
	if !ok1 || !ok2 {
		return nil, invalid("CSV header must include matric_no and full_name")
	}

	res := &ImportResult{Rejected: []RowError{}}
	seen := map[string]int{}
	var rows []models.Student
	line := 1
	for {
		rec, err := cr.Read()
		line++
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			res.Rejected = append(res.Rejected, RowError{Line: line, Error: "malformed CSV row"})
			continue
		}
		if line > maxRosterRows+1 {
			return nil, invalid("roster exceeds %d rows", maxRosterRows)
		}
		get := func(i int) string {
			if i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		matric := auth.NormalizeMatric(get(mi))
		name := strings.Join(strings.Fields(get(ni)), " ")
		switch {
		case matric == "":
			res.Rejected = append(res.Rejected, RowError{line, fmt.Sprintf("invalid matric_no %q (expected e.g. IFT/22/9203)", truncate(get(mi), 30))})
			continue
		case len([]rune(name)) < 2 || len([]rune(name)) > 120:
			res.Rejected = append(res.Rejected, RowError{line, "full_name must be 2–120 characters"})
			continue
		case seen[matric] > 0:
			res.Rejected = append(res.Rejected, RowError{line, fmt.Sprintf("duplicate of line %d", seen[matric])})
			continue
		}
		seen[matric] = line
		st := models.Student{MatricNo: matric, FullName: name}
		if hasDept {
			dept := strings.Join(strings.Fields(get(di)), " ")
			if len([]rune(dept)) > 120 {
				res.Rejected = append(res.Rejected, RowError{line, "department must be at most 120 characters"})
				continue
			}
			st.Department = dept
		}
		if hasPhoto {
			if p := get(pi); strings.HasPrefix(p, "blob:roster/") && store.ValidKey(p[5:]) {
				st.PhotoURL = p
			}
		}
		rows = append(rows, st)
	}
	if err := s.repo.UpsertStudents(ctx, rows); err != nil {
		return nil, err
	}
	res.Imported = len(rows)
	return res, nil
}

// SetPhoto validates that the image contains a face and stores it as the
// student's roster reference photo.
func (s *RosterService) SetPhoto(ctx context.Context, matcher *FaceMatcher, st *models.Student, data []byte) error {
	if len(data) == 0 || len(data) > MaxPhotoBytes {
		return ErrBadMedia
	}
	img, err := ai.Decode(data)
	if err != nil {
		return ErrBadMedia
	}
	if _, err := matcher.Embedding(img); err != nil {
		return err
	}
	// Store a re-encoded JPEG (max 800 px): consistent format, small files,
	// and no EXIF metadata such as GPS location from phone cameras.
	normalised, err := normalisePhoto(img)
	if err != nil {
		return err
	}
	key := "roster/" + strings.ReplaceAll(st.MatricNo, "/", "-") + ".jpg"
	if err := s.blobs.Put(ctx, key, normalised, "image/jpeg"); err != nil {
		return err
	}
	return s.repo.SetStudentPhoto(ctx, st.ID, "blob:"+key)
}

// MatricFromFilename maps "IFT-22-9203.jpg" to "IFT/22/9203" for bulk
// photo uploads.
func MatricFromFilename(name string) string {
	base := strings.TrimSuffix(path.Base(strings.ReplaceAll(name, "\\", "/")), path.Ext(name))
	return auth.NormalizeMatric(strings.ReplaceAll(base, "-", "/"))
}

// Erase permanently deletes a student and everything derived from them:
// sessions, answers, violations, AI findings (FK cascade) and every stored
// image (roster photo, verification captures, frames, audio).
func (s *RosterService) Erase(ctx context.Context, st *models.Student) error {
	sessions, err := s.repo.SessionsForStudent(ctx, st.ID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		for _, prefix := range []string{
			fmt.Sprintf("verify/%d/%d/", sess.ExamID, st.ID),
			fmt.Sprintf("frames/%d/%d/", sess.ExamID, sess.ID),
			fmt.Sprintf("audios/%d/%d/", sess.ExamID, sess.ID),
		} {
			if err := s.blobs.DeletePrefix(ctx, prefix); err != nil {
				return fmt.Errorf("delete media %s: %w", prefix, err)
			}
		}
	}
	if key := RosterPhotoKey(st); key != "" {
		if err := s.blobs.Delete(ctx, key); err != nil {
			return err
		}
	}
	return s.repo.DeleteStudent(ctx, st.ID)
}

// Export assembles everything held about a student (data-subject access).
func (s *RosterService) Export(ctx context.Context, st *models.Student) (map[string]any, error) {
	sessions, err := s.repo.SessionsForStudent(ctx, st.ID)
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, sess := range sessions {
		vs, err := s.repo.ViolationsForSession(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		ans, err := s.repo.Answers(ctx, sess.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"session": sess, "violations": vs, "answers": ans})
	}
	return map[string]any{"student": st, "sessions": out}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// normalisePhoto downscales to at most 800 px on the long side and encodes
// as JPEG (quality 90). Re-encoding drops all metadata.
func normalisePhoto(img image.Image) ([]byte, error) {
	const maxSide = 800
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxSide || h > maxSide {
		scale := float64(maxSide) / float64(max(w, h))
		nw, nh := int(float64(w)*scale), int(float64(h)*scale)
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		chw := ai.ToCHW(img, nw, nh, ai.Norm{Scale: 1, Std: [3]float32{1, 1, 1}})
		plane := nw * nh
		for i := 0; i < plane; i++ {
			dst.Pix[i*4] = clampByte(chw[i])
			dst.Pix[i*4+1] = clampByte(chw[plane+i])
			dst.Pix[i*4+2] = clampByte(chw[2*plane+i])
			dst.Pix[i*4+3] = 255
		}
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clampByte(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

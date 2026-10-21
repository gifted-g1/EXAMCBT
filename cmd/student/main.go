// Command student adds or updates one roster student and, optionally, their
// reference photo — without the web dashboard or a running server.
//
//	go run ./cmd/student -matric IFT/22/9203 -name "Awodele Kehinde Gabriel" \
//	    -department "Information Technology" -photo path/to/photo.png
//
// It uses the same configuration as the API (.env / environment), so in
// standalone mode it writes to data/examshield.db and data/blobs. The photo
// must contain exactly one clearly visible face.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gorm.io/gorm"

	"examshield/internal/ai"
	"examshield/internal/auth"
	"examshield/internal/config"
	"examshield/internal/models"
	"examshield/internal/repositories"
	"examshield/internal/services"
	"examshield/internal/store"
)

func main() {
	matric := flag.String("matric", "", "matric number, e.g. IFT/22/9203 (required)")
	name := flag.String("name", "", "full name (required for a new student; updates an existing one)")
	dept := flag.String("department", "", "department, e.g. Information Technology (optional)")
	photo := flag.String("photo", "", "reference photo, JPEG or PNG, one face (optional)")
	flag.Parse()
	if err := run(*matric, *name, *dept, *photo); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(matricIn, name, dept, photoPath string) error {
	matric := auth.NormalizeMatric(matricIn)
	if matric == "" {
		return fmt.Errorf("matric number %q must look like IFT/22/9203", matricIn)
	}
	name = strings.Join(strings.Fields(name), " ")
	dept = strings.Join(strings.Fields(dept), " ")
	if len([]rune(name)) > 120 || len([]rune(dept)) > 120 {
		return errors.New("name and department must be at most 120 characters")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	var db *gorm.DB
	if cfg.Mode == config.ModeStandalone {
		db, err = store.OpenSQLite(cfg.SQLitePath)
	} else {
		db, err = store.OpenMySQL(cfg.DBDSN, 2, 1)
	}
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	repo := repositories.New(db)

	existing, err := repo.StudentByMatric(ctx, matric)
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		if len([]rune(name)) < 2 {
			return errors.New("-name is required for a new student")
		}
		existing = nil
	case err != nil:
		return err
	}
	row := models.Student{MatricNo: matric, FullName: name, Department: dept}
	if existing != nil && name == "" {
		row.FullName = existing.FullName
	}
	if err := repo.UpsertStudents(ctx, []models.Student{row}); err != nil {
		return err
	}
	st, err := repo.StudentByMatric(ctx, matric)
	if err != nil {
		return err
	}
	verb := "added"
	if existing != nil {
		verb = "updated"
	}
	fmt.Printf("%s %s — %s", verb, st.MatricNo, st.FullName)
	if st.Department != "" {
		fmt.Printf(" (%s)", st.Department)
	}
	fmt.Println()

	if photoPath == "" {
		return nil
	}
	data, err := os.ReadFile(photoPath)
	if err != nil {
		return err
	}
	var blobs store.Blobs
	if cfg.MinIOEndpoint != "" {
		blobs, err = store.OpenMinIO(ctx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	} else {
		blobs, err = store.OpenDisk(cfg.BlobDir)
	}
	if err != nil {
		return fmt.Errorf("blob store: %w", err)
	}
	engine, err := ai.Load(ai.Options{Backend: cfg.AIBackend, ModelDir: cfg.ModelDir, ONNXLib: cfg.ONNXLibPath, Log: log},
		ai.Need{Faces: true, Embed: true})
	if err != nil {
		return err
	}
	defer engine.Close()
	matcher := services.NewFaceMatcher(engine, cfg.FaceMatchThreshold, cfg.FaceMatchEnforce)
	roster := services.NewRosterService(repo, blobs, log)
	switch err := roster.SetPhoto(ctx, matcher, st, data); {
	case errors.Is(err, services.ErrNoFace):
		return errors.New("no face found in the photo — use a clear, front-facing photo")
	case errors.Is(err, services.ErrMultipleFaces):
		return errors.New("more than one face in the photo — use a photo of the student alone")
	case errors.Is(err, services.ErrBadMedia):
		return errors.New("the photo must be a JPEG or PNG under 4 MB")
	case err != nil:
		return err
	}
	fmt.Printf("reference photo saved for %s\n", st.MatricNo)
	return nil
}

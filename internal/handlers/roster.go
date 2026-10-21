package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"examshield/internal/auth"
	"examshield/internal/httpx"
	"examshield/internal/models"
	"examshield/internal/repositories"
	"examshield/internal/services"
)

// ListStudents: GET /api/staff/students?q=&page=1&per_page=50
func (h *Handlers) ListStudents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	per, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if per < 1 || per > 200 {
		per = 50
	}
	rows, total, err := h.Repo.ListStudents(c.Request.Context(), c.Query("q"), per, (page-1)*per)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"students": rows, "total": total, "page": page, "per_page": per})
}

// CreateStudent: POST /api/staff/students {"matric_no","full_name"}
func (h *Handlers) CreateStudent(c *gin.Context) {
	var req struct {
		MatricNo   string `json:"matric_no"`
		FullName   string `json:"full_name"`
		Department string `json:"department"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "matric_no and full_name are required")
		return
	}
	matric := auth.NormalizeMatric(req.MatricNo)
	name := strings.Join(strings.Fields(req.FullName), " ")
	dept := strings.Join(strings.Fields(req.Department), " ")
	if matric == "" || len(name) < 2 || len(name) > 120 || len([]rune(dept)) > 120 {
		httpx.BadRequest(c, "matric_no must look like IFT/22/9203 and full_name 2–120 characters")
		return
	}
	if _, err := h.Repo.StudentByMatric(c.Request.Context(), matric); err == nil {
		httpx.Abort(c, http.StatusConflict, httpx.CodeConflict, "matric number already on the roster")
		return
	}
	st := &models.Student{MatricNo: matric, FullName: name, Department: dept}
	if err := h.Repo.CreateStudent(c.Request.Context(), st); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "student_create", matric, nil)
	c.JSON(http.StatusCreated, st)
}

// ImportStudents: POST /api/staff/students/import (multipart "file": CSV)
func (h *Handlers) ImportStudents(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "CSV file is required (field 'file', max 5 MB)")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer f.Close()
	res, err := h.Roster.ImportCSV(c.Request.Context(), f)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "roster_import", fh.Filename, map[string]any{"imported": res.Imported, "rejected": len(res.Rejected)})
	c.JSON(http.StatusOK, res)
}

// UploadPhoto: POST /api/staff/students/:id/photo (multipart "photo")
func (h *Handlers) UploadPhoto(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	st, err := h.Repo.StudentByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxPhotoBytes+64<<10)
	data, err := readPart(c, "photo")
	if err != nil {
		httpx.BadRequest(c, "photo is required (JPEG/PNG, max 4 MB)")
		return
	}
	if err := h.Roster.SetPhoto(c.Request.Context(), h.Matcher, st, data); err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "student_photo", st.MatricNo, nil)
	c.Status(http.StatusNoContent)
}

// BulkPhotos: POST /api/staff/students/photos (multipart "photos", files
// named like IFT-22-9203.jpg)
func (h *Handlers) BulkPhotos(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200<<20)
	form, err := c.MultipartForm()
	if err != nil {
		httpx.BadRequest(c, "multipart form with 'photos' files is required (max 200 MB)")
		return
	}
	type result struct {
		File   string `json:"file"`
		Status string `json:"status"`
	}
	var out []result
	for _, fh := range form.File["photos"] {
		matric := services.MatricFromFilename(fh.Filename)
		if matric == "" {
			out = append(out, result{fh.Filename, "filename is not a matric number (use IFT-22-9203.jpg)"})
			continue
		}
		st, err := h.Repo.StudentByMatric(c.Request.Context(), matric)
		if err != nil {
			out = append(out, result{fh.Filename, "not on roster"})
			continue
		}
		data, err := readFile(fh, services.MaxPhotoBytes)
		if err == nil {
			err = h.Roster.SetPhoto(c.Request.Context(), h.Matcher, st, data)
		}
		switch {
		case err == nil:
			out = append(out, result{fh.Filename, "ok"})
		case errors.Is(err, services.ErrNoFace):
			out = append(out, result{fh.Filename, "no face detected"})
		case errors.Is(err, services.ErrMultipleFaces):
			out = append(out, result{fh.Filename, "more than one face"})
		default:
			out = append(out, result{fh.Filename, "invalid image"})
		}
	}
	h.audit(c, "roster_photos", "", map[string]any{"files": len(out)})
	c.JSON(http.StatusOK, gin.H{"results": out})
}

// StudentPhoto: GET /api/staff/students/:id/photo
func (h *Handlers) StudentPhoto(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	st, err := h.Repo.StudentByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	h.serveBlob(c, services.RosterPhotoKey(st))
}

// ExportStudent: GET /api/staff/students/:id/export (admin) — subject access.
func (h *Handlers) ExportStudent(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	st, err := h.Repo.StudentByID(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	data, err := h.Roster.Export(c.Request.Context(), st)
	if err != nil {
		fail(c, err)
		return
	}
	h.audit(c, "student_export", st.MatricNo, nil)
	c.Header("Content-Disposition", `attachment; filename="student-`+strconv.FormatInt(st.ID, 10)+`.json"`)
	c.JSON(http.StatusOK, data)
}

// DeleteStudent: DELETE /api/staff/students/:id (admin) — right to erasure.
func (h *Handlers) DeleteStudent(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	st, err := h.Repo.StudentByID(c.Request.Context(), id)
	if errors.Is(err, repositories.ErrNotFound) {
		httpx.NotFound(c, "student")
		return
	}
	if err != nil {
		fail(c, err)
		return
	}
	if err := h.Roster.Erase(c.Request.Context(), st); err != nil {
		fail(c, err)
		return
	}
	// The audit row records that an erasure happened, not who it was about.
	h.audit(c, "student_erase", strconv.FormatInt(id, 10), nil)
	c.Status(http.StatusNoContent)
}

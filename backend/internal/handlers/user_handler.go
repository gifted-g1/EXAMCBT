package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	appmw "examshield/internal/middleware"
	"examshield/internal/models"
	"examshield/internal/repositories"
)

type UserHandler struct {
	users *repositories.UserRepository
}

func NewUserHandler(users *repositories.UserRepository) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := appmw.ClaimsFromContext(r.Context())
	if !ok {
		appmw.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var rolePtr *models.Role
	if roleParam := r.URL.Query().Get("role"); roleParam != "" {
		role := models.Role(roleParam)
		rolePtr = &role
	}
	users, err := h.users.ListByOrganization(r.Context(), claims.OrganizationID, rolePtr)
	if err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to list users")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, users)
}

func (h *UserHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	if err := h.users.SetSuspended(r.Context(), id, true); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to suspend account")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"message": "account suspended"})
}

func (h *UserHandler) Reinstate(w http.ResponseWriter, r *http.Request) {
	id, ok := mustParseUUID(w, mux.Vars(r)["id"])
	if !ok {
		return
	}
	if err := h.users.SetSuspended(r.Context(), id, false); err != nil {
		appmw.WriteError(w, http.StatusInternalServerError, "unable to reinstate account")
		return
	}
	appmw.WriteJSON(w, http.StatusOK, map[string]string{"message": "account reinstated"})
}

// Package proxy implements the admin API handler for runtime configuration management.
package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
)

// AdminHandler handles admin API requests for profile management.
type AdminHandler struct {
	handler *Handler
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(handler *Handler) *AdminHandler {
	return &AdminHandler{handler: handler}
}

// ServeHTTP handles admin API requests.
func (a *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow localhost requests for security
	host := r.Host
	if !strings.HasPrefix(host, "localhost:") && !strings.HasPrefix(host, "127.0.0.1:") {
		http.Error(w, "Admin API only accessible from localhost", http.StatusForbidden)
		return
	}

	// Verify admin token
	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		// Also check query parameter for convenience
		token = r.URL.Query().Get("token")
	}
	if token != a.handler.GetAdminToken() {
		http.Error(w, "Invalid admin token", http.StatusUnauthorized)
		return
	}

	// Route based on path
	path := r.URL.Path

	switch {
	case path == "/_admin/profiles" && r.Method == http.MethodGet:
		a.handleListProfiles(w, r)
	case path == "/_admin/profiles/active" && r.Method == http.MethodGet:
		a.handleGetActiveProfile(w, r)
	case path == "/_admin/profiles/switch" && r.Method == http.MethodPost:
		a.handleSwitchProfile(w, r)
	default:
		http.Error(w, "Unknown admin endpoint", http.StatusNotFound)
	}
}

// ProfileResponse represents a profile in API responses.
type ProfileResponse struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Routes      map[string]string `json:"routes"`
	IsActive    bool              `json:"isActive"`
}

// ListProfilesResponse represents the response for listing profiles.
type ListProfilesResponse struct {
	Profiles       []ProfileResponse `json:"profiles"`
	ActiveProfile  string            `json:"activeProfile"`
	HasProfiles    bool              `json:"hasProfiles"`
	LegacyRoutes   map[string]string `json:"legacyRoutes,omitempty"`
}

// handleListProfiles returns all configured profiles.
func (a *AdminHandler) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := a.handler.GetProfiles()
	activeProfile := a.handler.GetActiveProfile()
	cfg := a.handler.GetConfig()

	var profileList []ProfileResponse
	for key, profile := range profiles {
		profileList = append(profileList, ProfileResponse{
			Key:         key,
			Name:        profile.Name,
			Description: profile.Description,
			Routes:      profile.Routes,
			IsActive:    key == activeProfile,
		})
	}

	// Sort profiles by key for consistent ordering
	// (simple alphabetical sort)
	for i := 0; i < len(profileList); i++ {
		for j := i + 1; j < len(profileList); j++ {
			if profileList[i].Key > profileList[j].Key {
				profileList[i], profileList[j] = profileList[j], profileList[i]
			}
		}
	}

	response := ListProfilesResponse{
		Profiles:      profileList,
		ActiveProfile: activeProfile,
		HasProfiles:   len(profiles) > 0,
		LegacyRoutes:  cfg.Router.Routes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetActiveProfile returns the current active profile name.
func (a *AdminHandler) handleGetActiveProfile(w http.ResponseWriter, r *http.Request) {
	activeProfile := a.handler.GetActiveProfile()

	response := struct {
		ActiveProfile string `json:"activeProfile"`
		HasProfiles   bool   `json:"hasProfiles"`
	}{
		ActiveProfile: activeProfile,
		HasProfiles:   len(a.handler.GetProfiles()) > 0,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SwitchProfileRequest represents the request body for switching profiles.
type SwitchProfileRequest struct {
	Profile string `json:"profile"`
}

// SwitchProfileResponse represents the response for switching profiles.
type SwitchProfileResponse struct {
	Success       bool              `json:"success"`
	ActiveProfile string            `json:"activeProfile"`
	ProfileName   string            `json:"profileName,omitempty"`
	Routes        map[string]string `json:"routes,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// handleSwitchProfile switches to a different profile.
func (a *AdminHandler) handleSwitchProfile(w http.ResponseWriter, r *http.Request) {
	var req SwitchProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := SwitchProfileResponse{
			Success: false,
			Error:   "Invalid request body",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.Profile == "" {
		response := SwitchProfileResponse{
			Success: false,
			Error:   "Profile name is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Switch the profile
	err := a.handler.UpdateActiveProfile(req.Profile)
	if err != nil {
		response := SwitchProfileResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get the profile details for the response
	profiles := a.handler.GetProfiles()
	profile := profiles[req.Profile]

	response := SwitchProfileResponse{
		Success:       true,
		ActiveProfile: req.Profile,
		ProfileName:   profile.Name,
		Routes:        profile.Routes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type Handler struct {
	db *gorm.DB
}

type Target struct {
	Slug      string `json:"slug"`
	TargetURL string `json:"target_url"`
}

func main() {
	// Connect to PlanetScale database using DSN environment variable.
	db, err := gorm.Open(mysql.Open(os.Getenv("DSN")), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		log.Fatalf("failed to connect to PlanetScale: %v", err)
	}
	handler := NewHandler(db)

	// Start an HTTP API server.
	const addr = ":8080"
	log.Printf("successfully connected to PlanetScale, starting HTTP server on %q", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("failed to serve HTTP: %v", err)
	}
}

func NewHandler(db *gorm.DB) http.Handler {
	h := &Handler{db: db}
	r := mux.NewRouter()
	r.HandleFunc("/", handleIndex).Methods(http.MethodGet)
	r.HandleFunc("/api", handleIndex).Methods(http.MethodGet)
	r.HandleFunc("/api/seed", h.seedDatabase).Methods(http.MethodGet)
	r.HandleFunc("/api/get/{slug}", h.getTarget).Methods(http.MethodGet)
	r.HandleFunc("/api/create", h.putTarget).Methods(http.MethodPost)
	r.HandleFunc("/{slug}", h.redirectToTarget).Methods(http.MethodGet)
	return r
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, `to get started send a post request to https://u.gosh.ga/api/create/ with json body like this -- {"slug":"my_unique_slug", "target_url":"https://foo-bar.com/"}`)
}

func (h *Handler) seedDatabase(w http.ResponseWriter, r *http.Request) {
	if err := h.db.AutoMigrate(&Target{}); err != nil {
		http.Error(w, "failed to migrate ghub table", http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "Migrations and Seeding of database complete\n")
}

func (h *Handler) redirectToTarget(w http.ResponseWriter, r *http.Request) {
	var target Target
	result := h.db.First(&target, h.db.Where("slug = ?", mux.Vars(r)["slug"]))
	if result.Error != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, target.TargetURL, http.StatusMovedPermanently)
}

func (h *Handler) checkForSlug(slug string) bool {
	if err := h.db.Where("slug = ?", slug).First(&Target{}).Error; err != nil {
		return false
	}
	return true
}

func validateSlug(slug string) bool {
	return strings.TrimSpace(slug) != "" && strings.TrimSpace(slug) != "api" 
}

func validateURL(targetURL string) bool {
	_, err := url.ParseRequestURI(targetURL)
	return err == nil
}

func (h *Handler) getTarget(w http.ResponseWriter, r *http.Request) {
	var target Target
	result := h.db.First(&target, h.db.Where("slug = ?", mux.Vars(r)["slug"]))
	if result.Error != nil {
		http.NotFound(w, r)
		return
	}

	json.NewEncoder(w).Encode(target)
}

func (h *Handler) putTarget(w http.ResponseWriter, r *http.Request) {
	var target Target
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.checkForSlug(target.Slug) || !validateSlug(target.Slug) {
		if !validateURL(target.TargetURL) {
			http.Error(w, "Target must be a valid URL!", http.StatusBadRequest)
			return
		}
		http.Error(w, "Slug already exists, please try another one!", http.StatusBadRequest)
		return
	} else {
		if err := h.db.Create(&target).Error; err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	json.NewEncoder(w).Encode(target)
}

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"parfum/config"
	"parfum/internal/repository"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Handler struct {
	// Your existing fields
	cfg         *config.Config
	logger      *zap.Logger
	ctx         context.Context
	bot         *bot.Bot
	parfumeRepo *repository.ParfumeRepository
}

func NewHandler(cfg *config.Config, zapLogger *zap.Logger, ctx context.Context, db *sql.DB) *Handler {
	h := &Handler{
		cfg:         cfg,
		logger:      zapLogger,
		ctx:         ctx,
		parfumeRepo: repository.NewParfumeRepository(db),
	}

	return h
}

func (h *Handler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

}

// SetBot sets the bot instance for the handler
func (h *Handler) SetBot(b *bot.Bot) {
	h.bot = b
}

func (h *Handler) StartWebServer(ctx context.Context, b *bot.Bot) {
	h.SetBot(b)

	// Create required directories
	os.MkdirAll("./static", 0755)
	os.MkdirAll("./files", 0755)
	os.MkdirAll("./payments", 0755)
	os.MkdirAll("./photo", 0755)

	// CORS Middleware for all requests
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Handle preflight OPTIONS request
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// Apply CORS to all routes
	mux := http.NewServeMux()

	// Static files with CORS
	mux.Handle("/static/", corsMiddleware(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))))
	mux.Handle("/files/", corsMiddleware(http.StripPrefix("/files/", http.FileServer(http.Dir("./files/")))))
	mux.Handle("/photo/", corsMiddleware(http.StripPrefix("/photo/", http.FileServer(http.Dir("./photo/")))))

	mux.HandleFunc("/parfume", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/parfume.html"
		http.ServeFile(w, r, path)
	})

	// Admin perfume page
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/admin-parfume.html"
		http.ServeFile(w, r, path)
	})

	// NEW: Add Perfume Page
	mux.HandleFunc("/admin/add-perfume", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := "./static/admin-add-parfume.html"
		http.ServeFile(w, r, path)
	})

	// NEW: Update Perfume Page
	mux.HandleFunc("/admin/update-perfume", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := "./static/admin-update-parfume.html"
		http.ServeFile(w, r, path)
	})

	mux.HandleFunc("/prize", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/prize.html"
		http.ServeFile(w, r, path)
	})

	// Perfume API endpoints
	mux.HandleFunc("/api/parfumes", h.handleGetPerfumes)
	mux.HandleFunc("/api/parfume/", h.handleGetPerfume)
	mux.HandleFunc("/api/add-parfume", h.handleAddPerfume)
	mux.HandleFunc("/api/update-parfume/", h.handleUpdatePerfume)
	mux.HandleFunc("/api/delete-parfume/", h.handleDeletePerfume)
	mux.HandleFunc("/api/search-parfumes", h.handleSearchPerfumes)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"service":   "meily-bot-api",
			"version":   "2.0.0-enhanced",
		})
	})

	h.logger.Info("Starting web server", zap.String("port", h.cfg.Port))
	if err := http.ListenAndServe(h.cfg.Port, mux); err != nil {
		h.logger.Fatal("Failed to start web server", zap.Error(err))
	}
}

// Get all perfumes
func (h *Handler) handleGetPerfumes(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	perfumes, err := h.parfumeRepo.GetAll()
	if err != nil {
		h.logger.Error("Error getting perfumes", zap.Error(err))
		http.Error(w, "Error getting perfumes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perfumes)
}

// Get single perfume by ID
func (h *Handler) handleGetPerfume(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/parfume/")
	if path == "" {
		http.Error(w, "Perfume ID required", http.StatusBadRequest)
		return
	}

	perfume, err := h.parfumeRepo.GetByID(path)
	if err != nil {
		h.logger.Error("Error getting perfume", zap.Error(err))
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Perfume not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error getting perfume", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perfume)
}

// Add new perfume
func (h *Handler) handleAddPerfume(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Extract form data
	name := r.FormValue("name")
	sex := r.FormValue("sex")
	description := r.FormValue("description")
	priceStr := r.FormValue("price")

	if name == "" || sex == "" || description == "" || priceStr == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	price, err := strconv.Atoi(priceStr)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	// Validate sex value
	if sex != "Male" && sex != "Female" && sex != "Unisex" {
		http.Error(w, "Invalid sex value", http.StatusBadRequest)
		return
	}

	// Handle file upload
	var photoPath string
	file, fileHeader, err := r.FormFile("photo")
	if err == nil {
		defer file.Close()

		// Generate unique filename
		ext := filepath.Ext(fileHeader.Filename)
		filename := uuid.New().String() + ext
		photoPath = filename

		// Create photo file
		dst, err := os.Create(filepath.Join("./photo", filename))
		if err != nil {
			h.logger.Error("Error creating photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy file content
		_, err = io.Copy(dst, file)
		if err != nil {
			h.logger.Error("Error copying photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
	}

	// Create perfume object
	perfume := &repository.Product{
		NameParfume: name,
		Sex:         sex,
		Description: description,
		Price:       price,
		PhotoPath:   photoPath,
	}

	// Save to database
	err = h.parfumeRepo.Create(perfume)
	if err != nil {
		h.logger.Error("Error creating perfume", zap.Error(err))
		http.Error(w, "Error creating perfume", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Perfume created successfully",
		"id":      perfume.Id,
	})
}

// Update perfume
func (h *Handler) handleUpdatePerfume(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/update-parfume/")
	if path == "" {
		http.Error(w, "Perfume ID required", http.StatusBadRequest)
		return
	}

	// Get existing perfume
	existingPerfume, err := h.parfumeRepo.GetByID(path)
	if err != nil {
		h.logger.Error("Error getting perfume for update", zap.Error(err))
		http.Error(w, "Perfume not found", http.StatusNotFound)
		return
	}

	// Parse multipart form
	err = r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	// Extract form data
	name := r.FormValue("name")
	sex := r.FormValue("sex")
	description := r.FormValue("description")
	priceStr := r.FormValue("price")

	if name == "" || sex == "" || description == "" || priceStr == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	price, err := strconv.Atoi(priceStr)
	if err != nil {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	// Validate sex value
	if sex != "Male" && sex != "Female" && sex != "Unisex" {
		http.Error(w, "Invalid sex value", http.StatusBadRequest)
		return
	}

	// Handle file upload (optional for update)
	photoPath := existingPerfume.PhotoPath // Keep existing photo by default
	file, fileHeader, err := r.FormFile("photo")
	if err == nil {
		defer file.Close()

		// Delete old photo if exists
		if existingPerfume.PhotoPath != "" {
			oldPhotoPath := filepath.Join("./photo", existingPerfume.PhotoPath)
			os.Remove(oldPhotoPath) // Ignore errors
		}

		// Generate unique filename
		ext := filepath.Ext(fileHeader.Filename)
		filename := uuid.New().String() + ext
		photoPath = filename

		// Create photo file
		dst, err := os.Create(filepath.Join("./photo", filename))
		if err != nil {
			h.logger.Error("Error creating photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Copy file content
		_, err = io.Copy(dst, file)
		if err != nil {
			h.logger.Error("Error copying photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
	}

	// Update perfume object
	updatedPerfume := &repository.Product{
		Id:          existingPerfume.Id,
		NameParfume: name,
		Sex:         sex,
		Description: description,
		Price:       price,
		PhotoPath:   photoPath,
	}

	// Update in database
	err = h.parfumeRepo.Update(updatedPerfume)
	if err != nil {
		h.logger.Error("Error updating perfume", zap.Error(err))
		http.Error(w, "Error updating perfume", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Perfume updated successfully",
	})
}

// Delete perfume
func (h *Handler) handleDeletePerfume(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/delete-parfume/")
	if path == "" {
		http.Error(w, "Perfume ID required", http.StatusBadRequest)
		return
	}

	// Get perfume to delete photo file
	perfume, err := h.parfumeRepo.GetByID(path)
	if err != nil {
		h.logger.Error("Error getting perfume for deletion", zap.Error(err))
		http.Error(w, "Perfume not found", http.StatusNotFound)
		return
	}

	// Delete from database
	err = h.parfumeRepo.Delete(path)
	if err != nil {
		h.logger.Error("Error deleting perfume", zap.Error(err))
		http.Error(w, "Error deleting perfume", http.StatusInternalServerError)
		return
	}

	// Delete photo file if exists
	if perfume.PhotoPath != "" {
		photoPath := filepath.Join("./photo", perfume.PhotoPath)
		err := os.Remove(photoPath)
		if err != nil {
			h.logger.Warn("Error deleting photo file", zap.Error(err))
			// Don't fail the request if photo deletion fails
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Perfume deleted successfully",
	})
}

// Search perfumes
func (h *Handler) handleSearchPerfumes(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get search parameters
	query := r.URL.Query().Get("q")
	sex := r.URL.Query().Get("sex")
	minPriceStr := r.URL.Query().Get("min_price")
	maxPriceStr := r.URL.Query().Get("max_price")

	var minPrice, maxPrice int
	var err error

	if minPriceStr != "" {
		minPrice, err = strconv.Atoi(minPriceStr)
		if err != nil {
			minPrice = 0
		}
	}

	if maxPriceStr != "" {
		maxPrice, err = strconv.Atoi(maxPriceStr)
		if err != nil {
			maxPrice = 0
		}
	}

	var perfumes []repository.Product

	if query != "" || sex != "" || minPrice > 0 || maxPrice > 0 {
		// Use advanced search
		perfumes, err = h.parfumeRepo.AdvancedSearch(query, sex, minPrice, maxPrice)
	} else {
		// Get all if no filters
		perfumes, err = h.parfumeRepo.GetAll()
	}

	if err != nil {
		h.logger.Error("Error searching perfumes", zap.Error(err))
		http.Error(w, "Error searching perfumes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perfumes)
}

// setCORSHeaders sets CORS headers for HTTP responses
func (h *Handler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

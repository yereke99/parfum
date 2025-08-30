package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"parfum/config"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

type Handler struct {
	// Your existing fields
	cfg    *config.Config
	logger *zap.Logger
	ctx    context.Context
	bot    *bot.Bot
}

func NewHandler(cfg *config.Config, zapLogger *zap.Logger, ctx context.Context) *Handler {
	h := &Handler{
		cfg:    cfg,
		logger: zapLogger,
		ctx:    ctx,
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

	mux.HandleFunc("/prize", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/prize.html"
		http.ServeFile(w, r, path)
	})
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

	if err := http.ListenAndServe(h.cfg.Port, mux); err != nil {
		h.logger.Fatal("Failed to start web server", zap.Error(err))
	}
}

// setCORSHeaders sets CORS headers for HTTP responses
func (h *Handler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

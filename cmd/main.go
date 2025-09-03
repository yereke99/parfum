package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"parfum/config"
	"parfum/internal/handler"
	"parfum/traits/database"
	"parfum/traits/logger"
	"syscall"
	"time"

	"github.com/go-telegram/bot"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	zapLogger, err := logger.NewLogger()
	if err != nil {
		panic(err)
	}

	zapLogger.Info("🌟 Starting Lumen Perfume Application...")

	// Initialize configuration
	cfg, err := config.NewConfig()
	if err != nil {
		zapLogger.Fatal("Failed to initialize config", zap.Error(err))
		return
	}

	// Initialize database
	db, err := sql.Open("sqlite3", cfg.DBName)
	if err != nil {
		zapLogger.Fatal("Failed to connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	// Test database connection
	if err = db.Ping(); err != nil {
		zapLogger.Fatal("Failed to ping database", zap.Error(err))
		return
	}

	zapLogger.Info("Database connected successfully", zap.String("db", cfg.DBName))

	// Create database tables
	if err := database.CreateTables(db); err != nil {
		zapLogger.Fatal("Failed to create database tables", zap.Error(err))
		return
	}

	// Create database views
	if err := database.CreateViews(db); err != nil {
		zapLogger.Warn("Failed to create database views", zap.Error(err))
	}

	// Run database migrations
	if err := database.MigrateDatabase(db); err != nil {
		zapLogger.Warn("Failed to run database migrations", zap.Error(err))
	}

	// Optionally seed sample data (only in development)
	if os.Getenv("LUMEN_ENV") != "production" {
		if err := database.SeedData(db); err != nil {
			zapLogger.Warn("Failed to seed sample data", zap.Error(err))
		}
	}

	zapLogger.Info("Database setup completed successfully")

	// Initialize context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize handler with database repositories
	handle := handler.NewHandler(cfg, zapLogger, ctx, db)

	// Initialize Telegram bot
	var b *bot.Bot
	if cfg.Token != "" {
		opts := []bot.Option{
			bot.WithDefaultHandler(handle.DefaultHandler),
		}

		b, err = bot.New(cfg.Token, opts...)
		if err != nil {
			zapLogger.Fatal("Failed to initialize Telegram bot", zap.Error(err))
			return
		}
		zapLogger.Info("Telegram bot initialized successfully")
	} else {
		zapLogger.Warn("No Telegram bot token provided, running without bot integration")
	}

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start web server in a goroutine
	go func() {
		zapLogger.Info("Starting Lumen web server", zap.String("port", cfg.Port))
		handle.StartWebServer(ctx, b)
	}()

	// Start Telegram bot if available
	if b != nil {
		go func() {
			zapLogger.Info("Starting Telegram bot...")
			b.Start(ctx)
		}()
	}

	// Optional: Start cleanup routine
	go func() {
		cleanupTicker := time.NewTicker(24 * time.Hour)
		defer cleanupTicker.Stop()
		for {
			select {
			case <-cleanupTicker.C:
				if err := database.CleanupOldData(db, 30); err != nil {
					zapLogger.Error("Failed to cleanup old data", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-stop
	zapLogger.Info("🛑 Shutdown signal received, gracefully stopping Lumen application...")
	cancel()

	// Close database connection
	if err := db.Close(); err != nil {
		zapLogger.Error("Error closing database connection", zap.Error(err))
	}

	zapLogger.Info("✅ Lumen application stopped gracefully")
}

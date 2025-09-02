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

	"github.com/go-telegram/bot"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func main() {
	zapLogger, err := logger.NewLogger()
	if err != nil {
		panic(err)
	}

	cfg, err := config.NewConfig()
	if err != nil {
		zapLogger.Error("error initializing config", zap.Error(err))
		return
	}

	db, err := sql.Open("sqlite3", cfg.DBName)
	if err != nil {
		zapLogger.Error("error in connect to database", zap.Error(err))
		return
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		zapLogger.Error("error pinging database", zap.Error(err))
		return
	}

	// Create tables
	if err := database.CreateTables(db); err != nil {
		zapLogger.Error("error creating tables", zap.Error(err))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	handle := handler.NewHandler(cfg, zapLogger, ctx, db)

	opts := []bot.Option{}
	b, err := bot.New(cfg.Token, opts...)
	if err != nil {
		zapLogger.Error("error in start bot", zap.Error(err))
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT)
	go func() {
		<-stop
		zapLogger.Info("Bot stopped successfully")
		cancel()
	}()

	go handle.StartWebServer(ctx, b)

	zapLogger.Info("Starting web server", zap.String("port", cfg.Port))
	zapLogger.Info("Bot started successfully")
	zapLogger.Info("Admin panel available at: /admin")
	b.Start(ctx)
}

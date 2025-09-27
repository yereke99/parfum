package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"parfum/config"
	"parfum/internal/domain"
	"parfum/internal/repository"
	"parfum/internal/service"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	StateStart   = "state_start"
	StateDefault = "state_default"
	StateCount   = "state_count"
	StatePay     = "state_pay"
	StateContact = "state_contact"
)

type Handler struct {
	cfg         *config.Config
	logger      *zap.Logger
	ctx         context.Context
	bot         *bot.Bot
	parfumeRepo *repository.ParfumeRepository
	clientRepo  *repository.ClientRepository
	orderRepo   *repository.OrderRepository
	redisRepo   *repository.RedisRepository
}

type Client struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	FIO        string `json:"fio"`
	Contact    string `json:"contact"`
	Address    string `json:"address"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type Order struct {
	ID          int64  `json:"id"`
	TelegramID  int64  `json:"telegram_id"`
	ClientID    int64  `json:"client_id"`
	CartData    string `json:"cart_data"`
	TotalAmount int    `json:"total_amount"`
	Status      string `json:"status"`
	PaymentLink string `json:"payment_link"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CartItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Quantity int    `json:"quantity"`
}


// Prize types
const (
	Prize10ML    = "parfum_10ml"
	Prize30ML    = "parfum_30ml" 
	PrizeDiamond = "diamond_ring"
	PrizeMoney   = "money"
)

// Prize wheel spin request/response
type SpinWheelRequest struct {
	TelegramID int64 `json:"telegram_id"`
}

type SpinWheelResponse struct {
	Success   bool   `json:"success"`
	CanSpin   bool   `json:"can_spin"`
	PrizeWon  string `json:"prize_won,omitempty"`
	Message   string `json:"message"`
	OrderID   int64  `json:"order_id,omitempty"`
	SpinsLeft int    `json:"spins_left"`
}

// Prize completion request
type CompletePrizeRequest struct {
	TelegramID int64  `json:"telegram_id"`
	OrderID    int64  `json:"order_id"`
	FIO        string `json:"fio"`
	Contact    string `json:"contact"`
	Address    string `json:"address"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
}

func NewHandler(cfg *config.Config, zapLogger *zap.Logger, ctx context.Context, db *sql.DB, redisClient *redis.Client) *Handler {
	h := &Handler{
		cfg:         cfg,
		logger:      zapLogger,
		ctx:         ctx,
		redisRepo:   repository.NewRedisRepository(redisClient),
		parfumeRepo: repository.NewParfumeRepository(db),
		clientRepo:  repository.NewClientRepository(db),
		orderRepo:   repository.NewOrderRepository(db),
	}

	return h
}


// Deterministic prize algorithm based on order sequence number
func (h *Handler) DeterminePrize(orderSequence int) string {
	// Every 200th order gets money (highest priority)
	if orderSequence%200 == 0 {
		return PrizeMoney
	}

	// Diamond rings: try to place at multiples of 100, with collision handling
	// We want 10 diamonds in first 1000 orders (1% rate)
	if orderSequence%100 == 0 {
		// This should be a diamond position, but check if it conflicts with money
		if orderSequence%200 != 0 {
			return PrizeDiamond
		}
	}

	// Handle diamond shifting for collision cases
	// If we're at a diamond position that conflicts with money,
	// we need to shift diamonds to nearby positions
	diamondPositions := []int{50, 150, 250, 350, 450, 550, 650, 750, 850, 950}
	for _, pos := range diamondPositions {
		if orderSequence == pos {
			return PrizeDiamond
		}
	}

	// Every 30th order gets 30ml (if not already taken by higher priority)
	if orderSequence%30 == 0 {
		// Check if this position is not taken by money or diamond
		if orderSequence%200 != 0 && orderSequence%100 != 0 {
			isDiamondPosition := false
			for _, pos := range diamondPositions {
				if orderSequence == pos {
					isDiamondPosition = true
					break
				}
			}
			if !isDiamondPosition {
				return Prize30ML
			}
		}
	}

	// All remaining orders get 10ml (should be ~90%)
	return Prize10ML
}

// Check if user can spin the wheel
func (h *Handler) CheckSpinEligibility(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telegramIDStr := r.URL.Query().Get("telegram_id")
	if telegramIDStr == "" {
		http.Error(w, "telegram_id parameter required", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram_id", http.StatusBadRequest)
		return
	}

	// Get user's orders that are paid but not yet completed with prizes
	orders, err := h.orderRepo.GetUnpaidOrdersByUser(telegramID)
	if err != nil {
		h.logger.Error("Error getting user orders", zap.Error(err))
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	availableSpins := 0
	var eligibleOrders []map[string]interface{}

	for _, order := range orders {
		// Count orders that have perfume selections but no prize yet
		if order.Parfumes != "" && (order.Gift == "" || order.Gift == "null") {
			availableSpins++
			eligibleOrders = append(eligibleOrders, map[string]interface{}{
				"id":         order.ID,
				"quantity":   order.Quantity,
				"parfumes":   order.Parfumes,
				"created_at": order.CreatedAt,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"can_spin":        availableSpins > 0,
		"spins_available": availableSpins,
		"eligible_orders": eligibleOrders,
	})
}

// Spin the wheel and determine prize
func (h *Handler) SpinWheel(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpinWheelRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.TelegramID == 0 {
		http.Error(w, "telegram_id required", http.StatusBadRequest)
		return
	}

	// Get user's eligible orders (paid, with perfumes, but no prize yet)
	orders, err := h.orderRepo.GetUnpaidOrdersByUser(req.TelegramID)
	if err != nil {
		h.logger.Error("Error getting user orders", zap.Error(err))
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var eligibleOrder *repository.Order
	for _, order := range orders {
		if order.Parfumes != "" && (order.Gift == "" || order.Gift == "null") {
			eligibleOrder = &order
			break
		}
	}

	if eligibleOrder == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SpinWheelResponse{
			Success: false,
			CanSpin: false,
			Message: "No eligible orders for spinning",
		})
		return
	}

	// Get global order sequence number for deterministic prize
	orderSequence, err := h.orderRepo.GetOrderSequenceNumber(eligibleOrder.ID)
	if err != nil {
		h.logger.Error("Error getting order sequence", zap.Error(err))
		// Fallback to order ID if sequence lookup fails
		orderSequence = int(eligibleOrder.ID)
	}

	// Determine prize using our algorithm
	prizeWon := h.DeterminePrize(orderSequence)

	// Save the prize to the order
	err = h.orderRepo.UpdateOrderPrize(eligibleOrder.ID, prizeWon)
	if err != nil {
		h.logger.Error("Error saving prize to order", zap.Error(err))
		http.Error(w, "Error saving prize", http.StatusInternalServerError)
		return
	}

	// Count remaining spins
	remainingSpins := 0
	for _, order := range orders {
		if order.ID != eligibleOrder.ID && order.Parfumes != "" && (order.Gift == "" || order.Gift == "null") {
			remainingSpins++
		}
	}

	h.logger.Info("Prize wheel spin completed",
		zap.Int64("telegram_id", req.TelegramID),
		zap.Int64("order_id", eligibleOrder.ID),
		zap.Int("order_sequence", orderSequence),
		zap.String("prize_won", prizeWon),
		zap.Int("remaining_spins", remainingSpins))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpinWheelResponse{
		Success:   true,
		CanSpin:   true,
		PrizeWon:  prizeWon,
		OrderID:   eligibleOrder.ID,
		SpinsLeft: remainingSpins,
		Message:   "Prize determined successfully",
	})
}

// Complete prize order with address information
func (h *Handler) CompletePrizeOrder(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	telegramIDStr := r.FormValue("telegram_id")
	orderIDStr := r.FormValue("order_id")
	fio := r.FormValue("fio")
	contact := r.FormValue("contact")
	address := r.FormValue("address")
	latitudeStr := r.FormValue("latitude")
	longitudeStr := r.FormValue("longitude")

	if telegramIDStr == "" || orderIDStr == "" || fio == "" || contact == "" || address == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram_id", http.StatusBadRequest)
		return
	}

	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid order_id", http.StatusBadRequest)
		return
	}

	// Get the order to verify it belongs to the user and has a prize
	order, err := h.orderRepo.GetByID(orderID)
	if err != nil {
		h.logger.Error("Error getting order", zap.Error(err))
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	if order.ID_user != telegramID {
		http.Error(w, "Order does not belong to user", http.StatusForbidden)
		return
	}

	if order.Gift == "" || order.Gift == "null" {
		http.Error(w, "Order has no prize assigned", http.StatusBadRequest)
		return
	}

	// Update the order with client information
	err = h.orderRepo.UpdateClientInfoWithCoordinates(orderID, fio, contact, address)
	if err != nil {
		h.logger.Error("Error updating order with client info", zap.Error(err))
		http.Error(w, "Error saving client information", http.StatusInternalServerError)
		return
	}

	// Mark order as completed
	err = h.orderRepo.MarkOrderAsCompleted(orderID)
	if err != nil {
		h.logger.Error("Error marking order as completed", zap.Error(err))
		// Don't fail the request, just log the error
	}

	// Send confirmation messages
	go h.sendPrizeCompletionMessages(telegramID, orderID, order.UserName, order.Gift, order.Parfumes, fio, contact, address)

	h.logger.Info("Prize order completed",
		zap.Int64("telegram_id", telegramID),
		zap.Int64("order_id", orderID),
		zap.String("prize", order.Gift),
		zap.String("fio", fio),
		zap.String("contact", contact),
		zap.String("address", address))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Prize order completed successfully",
		"prize":   order.Gift,
	})
}

// Send prize completion messages to user and admin
func (h *Handler) sendPrizeCompletionMessages(telegramID, orderID int64, userName, prize, parfumes, fio, contact, address string) {
	if h.bot == nil {
		h.logger.Error("Bot not initialized")
		return
	}

	// Get prize display names
	prizeNames := map[string]string{
		Prize10ML:    "🧪 10мл парфюм",
		Prize30ML:    "🧪 30мл парфюм", 
		PrizeDiamond: "💍 Бриллиант сақина",
		PrizeMoney:   "💰 100,000 теңге",
	}

	prizeDisplay := prizeNames[prize]
	if prizeDisplay == "" {
		prizeDisplay = prize
	}

	// User confirmation message
	userMessage := fmt.Sprintf(
		"🎉 Құттықтаймыз! Сіз сыйлық ұттыңыз! 🎉\n\n"+
			"🏆 Сіздің сыйлығыңыз: %s\n\n"+
			"📦 Тапсырыс мәліметтері:\n"+
			"🆔 Тапсырыс №: %d\n"+
			"👤 Тапсырыс беруші: %s\n"+
			"📱 Телефон: %s\n"+
			"📍 Мекенжай: %s\n\n"+
			"🌸 Таңдалған парфюмдер:\n%s\n\n"+
			"🚚 Жеткізу туралы ақпарат:\n"+
			"Біздің менеджер сізбен 24 сағат ішінде байланысады.\n"+
			"Сыйлығыңыз парфюммен бірге жеткізіледі.\n\n"+
			"Рахмет! 💝",
		prizeDisplay, orderID, fio, contact, address, parfumes)

	// Send to user
	_, err := h.bot.SendMessage(h.ctx, &bot.SendMessageParams{
		ChatID: telegramID,
		Text:   userMessage,
	})

	if err != nil {
		h.logger.Error("Failed to send prize completion message to user",
			zap.Error(err),
			zap.Int64("telegram_id", telegramID))
	}

	// Admin notification message
	adminMessage := fmt.Sprintf(
		"🎊 ЖАҢА СЫЙЛЫҚ ЖЕҢІМПАЗЫ! 🎊\n\n"+
			"🏆 Сыйлық: %s\n"+
			"🆔 Тапсырыс: %d\n"+
			"👤 Клиент: %s (@%s)\n"+
			"📱 Телефон: %s\n"+
			"📍 Мекенжай: %s\n"+
			"🌸 Парфюмдер: %s\n"+
			"⏰ Уақыт: %s\n\n"+
			"⚠️ СЫЙЛЫҚТЫ ПАРФЮММЕН БІРГЕ ЖЕТКІЗУ КЕРЕК!",
		prizeDisplay, orderID, fio, userName, contact, address, parfumes,
		time.Now().Format("2006-01-02 15:04:05"))

	// Send to admins
	admins := []int64{h.cfg.AdminID, h.cfg.AdminID2}
	for _, adminID := range admins {
		if adminID != 0 {
			_, err := h.bot.SendMessage(h.ctx, &bot.SendMessageParams{
				ChatID: adminID,
				Text:   adminMessage,
			})
			if err != nil {
				h.logger.Error("Failed to send admin prize notification",
					zap.Error(err),
					zap.Int64("admin_id", adminID))
			}
		}
	}
}

func (h *Handler) StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	promoText := "24990тгге 30мл парфюм сатып алып, 10мл, 30мллік парфюм , 89990тглік бриллант жүзік және 100 000 теңге ақшалай сыйлықтың біріне ие болыңыз."

	inlineKbd := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "🛍 Сатып алу",
					CallbackData: "buy_parfume",
				},
			},
		},
	}
	_, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:         update.Message.Chat.ID,
		Photo:          &models.InputFileString{Data: h.cfg.StartPhotoId},
		Caption:        promoText,
		ReplyMarkup:    inlineKbd,
		ProtectContent: true,
	})
	if err != nil {
		h.logger.Warn("Failed to send promo photo", zap.Error(err))
	}
}

func (h *Handler) DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	var userId int64
	if update.Message != nil {
		userId = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		userId = update.CallbackQuery.From.ID
	}

	ok, errE := h.clientRepo.ExistsJust(ctx, userId)
	if errE != nil {
		h.logger.Error("Failed to check user", zap.Error(errE))
	} else if !ok {
		timeNow := time.Now().Format("2006-01-02 15:04:05")
		h.logger.Info("New user", zap.String("user_id", strconv.FormatInt(userId, 10)), zap.String("date", timeNow))
		if errN := h.clientRepo.InsertJust(ctx, domain.JustEntry{
			UserId:         userId,
			UserName:       update.Message.From.Username,
			DateRegistered: timeNow,
		}); errN != nil {
			h.logger.Error("Failed to insert user", zap.Error(errN))
		}
	}

	if userId == h.cfg.AdminID {
		var fileId string
		switch {
		case len(update.Message.Photo) > 0:
			fileId = update.Message.Photo[len(update.Message.Photo)-1].FileID
		case update.Message.Video != nil:
			fileId = update.Message.Video.FileID
		}
		if fileId != "" {
			_, err := b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: h.cfg.AdminID,
				Text:   fileId,
			})
			if err != nil {
				h.logger.Error("error send fileId to admin", zap.Error(err))
			}
		}
	}

	userState := h.getOrCreateUserState(ctx, userId)
	if update.Message.Document != nil {
		if userState.State != StatePay && userState.State != StateContact {
			h.logger.Info("Document message", zap.String("user_id", strconv.FormatInt(update.Message.From.ID, 10)))
			//h.JustPaid(ctx, b, update)
			return
		}
	}

	fmt.Println("UserState: ", userState.State)
	
	if update.CallbackQuery != nil {
		switch userState.State {
		case StateStart:
			h.StartHandler(ctx, b, update)
			return
		case StateDefault:
			h.DefaultHandler(ctx, b, update)
			return
		case StateCount:
			h.CountHandler(ctx, b, update)
			return
		case StatePay:
			h.PaidHandler(ctx, b, update)
			return
		case StateContact:
			h.ShareContactCallbackHandler(ctx, b, update)
			return
		}
	}

	switch userState.State {
	case StateStart:
		h.StartHandler(ctx, b, update)
		return
	case StateDefault:
		h.DefaultHandler(ctx, b, update)
		return
	case StateCount:
		h.CountHandler(ctx, b, update)
		return
	case StatePay:
		h.PaidHandler(ctx, b, update)
		return
	case StateContact:
		h.ShareContactCallbackHandler(ctx, b, update)
		return
	default:
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		    ChatID: update.Message.Chat.ID,
		    Text:   "Welcome to Parfum Bot!",
	    })
	    if err != nil {
		    h.logger.Error("failed to send message", zap.Error(err))
	    }
	}
	
}

func (h *Handler) BuyParfumeHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil || update.CallbackQuery.Data != "buy_parfume" {
		return
	}

	userId := update.CallbackQuery.From.ID
	newState := &domain.UserState{
		State:  StateCount,
		Count:  0,
		IsPaid: false,
	}
	if err := h.redisRepo.SaveUserState(ctx, userId, newState); err != nil {
		h.logger.Error("Failed to save user state to Redis", zap.Error(err))
	}

	rows := make([][]models.InlineKeyboardButton, 6)
	for i := 0; i < 6; i++ {
		row := make([]models.InlineKeyboardButton, 5)
		for j := 0; j < 5; j++ {
			num := 5*i + j + 1
			row[j] = models.InlineKeyboardButton{
				Text:         strconv.Itoa(num),
				CallbackData: fmt.Sprintf("count_%d", num),
			}
		}
		rows[i] = row
	}

	btn := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})
	if err != nil {
		h.logger.Warn("Failed to answer callback query", zap.Error(err))
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      userId,
		Text:        "🧪 Парфюм санын таңдаңыз",
		ReplyMarkup: btn,
	})
	if err != nil {
		h.logger.Warn("Failed to answer callback query", zap.Error(err))
	}
}

func (h *Handler) CountHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil || !strings.HasPrefix(update.CallbackQuery.Data, "count_") {
		return
	}

	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
	})

	choice := strings.Split(update.CallbackQuery.Data, "_")
	if len(choice) != 2 {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
		})
		return
	}

	userCount, err := strconv.Atoi(choice[1])
	if err != nil {
		h.logger.Warn("Failed to parse count", zap.Error(err))
		return
	}

	totalSum := h.cfg.Cost * userCount

	userId := update.CallbackQuery.From.ID
	newState := &domain.UserState{
		State:  StatePay,
		Count:  userCount,
		IsPaid: false,
	}
	if err := h.redisRepo.SaveUserState(ctx, userId, newState); err != nil {
		h.logger.Warn("Failed to save user state in count handler", zap.Error(err))
	}

	inlineKbd := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "💳 Төлем жасау",
					URL:  "https://pay.kaspi.kz/pay/xopyuql9",
				},
			},
		},
	}
	msgTxt := fmt.Sprintf("✅ Тамаша! Енді төмендегі сілтемеге өтіп %d теңге төлем жасап, төлемді растайтын чекті PDF форматында ботқа кері жіберіңіз.", totalSum)
	_, sendErr := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      userId,
		Text:        msgTxt,
		ReplyMarkup: inlineKbd,
	})
	if sendErr != nil {
		h.logger.Warn("Failed to send confirmation message", zap.Error(sendErr))
	}
}

func (h *Handler) PaidHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.Document == nil {
		return
	}

	doc := update.Message.Document
	if !strings.EqualFold(filepath.Ext(doc.FileName), ".pdf") {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.From.ID,
			Text:   "❌ Қате! Тек қана PDF 📄 форматындағы файлдарды қабылдаймыз.",
		})
		return
	}

	userId := update.Message.From.ID
	fileInfo, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: doc.FileID,
	})
	if err != nil {
		h.logger.Error("Failed to get file info", zap.Error(err))
		return
	}

	fileUrl := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", h.cfg.Token, fileInfo.FilePath)
	resp, err := http.Get(fileUrl)
	if err != nil {
		h.logger.Error("Failed to download file via HTTP", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	saveDir := h.cfg.SavePaymentsDir
	if err := os.Mkdir(saveDir, 0755); err != nil {
		h.logger.Error("Failed to create payments directory", zap.Error(err))
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("%d_%s.pdf", userId, timestamp)
	savePath := filepath.Join(saveDir, fileName)

	outFile, err := os.Create(savePath)
	if err != nil {
		h.logger.Error("Failed to create file on disk", zap.Error(err))
		return
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		h.logger.Error("Failed to save PDF file", zap.Error(err))
		return
	}
	h.logger.Info("PDF file saved", zap.String("path", savePath))

	result, err := service.ReadPDF(savePath)
	if err != nil {
		h.logger.Warn("Failed to read PDF file", zap.Error(err))
	}
	if len(result) < 4 {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "❌ Дұрыс емес форматтағы чек! 📄 Қайталап көріңіз.",
		})
		return
	}

	h.logger.Info("PDF file read", zap.Any("result", result))

	ok, err := h.clientRepo.IsUniqueQr(ctx, result[3])
	if err != nil {
		h.logger.Error("error in check unique", zap.Error(err))
		return
	}
	if ok {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "⚠️ Бұл чек бұрын төленіп қойылған! 💳 ✅",
		})
		return
	}

	var pdfPrice, qrPdf string
	pdfPrice = result[2]
	qrPdf = result[3]
	bin, _ := service.ParsePrice(result[4])
	if result[0] == "Платеж успешно совершен" {
		pdfPrice = result[1]
		qrPdf = result[2]
		bin, _ = service.ParsePrice(result[3])
	}

	actualPrice, err := service.ParsePrice(pdfPrice)
	if err != nil {
		h.logger.Error("Failed to parse price from PDF file", zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: userId,
			Text:   "❌ Дұрыс емес PDF файл! 📄 Қайталап көріңіз.",
		})
		return
	}

	state, err := h.redisRepo.GetUserState(ctx, userId)
	if err != nil {
		h.logger.Error("Failed to get user state from Redis", zap.Error(err))
		return
	}

	rows := make([][]models.InlineKeyboardButton, 6)
	for i := 0; i < 6; i++ {
		row := make([]models.InlineKeyboardButton, 5)
		for j := 0; j < 5; j++ {
			num := i*5 + j + 1
			row[j] = models.InlineKeyboardButton{
				Text:         strconv.Itoa(num),
				CallbackData: fmt.Sprintf("count_%d", num),
			}
		}
		rows[i] = row
	}

	btn := &models.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	for i := 2400; i < 2500; i++ {
		if actualPrice == i {
			actualPrice = 2499
			break
		}
	}
	totalPrice := state.Count * h.cfg.Cost
	predictedCount := actualPrice / h.cfg.Cost
	textPrice := fmt.Sprintf("⚠️ Дұрыс емес сумма! 💰\n\n🔄 Көрсетілген сумаға сәйкес төлеңіз!\n📦 Немесе жиынтық суммасына сәйкес жиынтық санын түймелер таңдаңыз.\n\nСіздң жиынтық саны: %d", predictedCount)
	if totalPrice != actualPrice {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      userId,
			Text:        textPrice,
			ReplyMarkup: btn,
		})
		return
	}

	totalLoto := state.Count * 3
	pdfResult := domain.PdfResult{
		Total:       state.Count,
		ActualPrice: actualPrice,
		Qr:          qrPdf,
		Bin:         bin,
	}

	if err := service.Validator(h.cfg, pdfResult); err != nil {
		h.logger.Error("error in save newState to redis", zap.Error(err))

		var errorMessage string
		if errors.Is(err, service.ErrWrongBin) {
			// Specific message for wrong BIN in Kazakh with emojis
			errorMessage = "❌ Қате банк картасы! 💳\n\n" +
				"🏦 Тек біздің серіктес банк картасымен төлем жасауға болады.\n" +
				"📋 Дұрыс банк картасын пайдаланып қайталап көріңіз!"
		} else if errors.Is(err, service.ErrWrongPrice) {
			// Message for wrong price
			errorMessage = "❌ Дұрыс емес сумма! 💰\n\n" +
				"🔍 Төлем сомасы сәйкес келмейді.\n" +
				"📄 Чекті қайталап тексеріп көріңіз!"
		} else {
			// Generic error message
			errorMessage = "❌ Дұрыс емес PDF файл! 📄\n\n" +
				"🔄 Қайталап көріңіз немесе жаңа чек жүктеңіз."
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: userId,
			Text:   errorMessage,
		})
		return
	}

	if state != nil {
		state.IsPaid = true
		state.State = StateContact
		if err := h.redisRepo.SaveUserState(ctx, userId, state); err != nil {
			h.logger.Error("Failed to save user state to Redis", zap.Error(err))
		}
	}

	// Just incrFease the total sum
	if err := h.clientRepo.IncreaseTotalSum(ctx, actualPrice); err != nil {
		h.logger.Error("Failed to increase total sum", zap.Error(err))
	}

	tickets := make([]int, 0, totalLoto)
	for i := 0; i < totalLoto; i++ {
		lotoId := rand.Intn(90000000) + 10000000
		if err := h.clientRepo.InsertLoto(ctx, domain.LotoEntry{
			UserID:  userId,
			LotoID:  lotoId,
			QR:      qrPdf,
			Receipt: savePath,
			DatePay: time.Now().Format("2006-01-02 15:04:05"),
			Checks:  false,
		}); err != nil {
			h.logger.Error("error in insert loto", zap.Error(err))
			return
		}
		tickets = append(tickets, lotoId)
	}

	f, errFile := os.Open(savePath)
	if errFile != nil {
		h.logger.Error("Failed to open file on disk", zap.Error(errFile))
	}
	// Enhanced message with emojis and better formatting
	msgText := fmt.Sprintf(
		"✅ Сәтті төлем жасалды! 🎉\n\n"+
			"👤 UserId: %d\n"+
			"🧴 Косметика саны: %d\n"+
			"💰 Төлем суммасы: %d ₸\n"+
			"📅 Уақыт: %s\n"+
			"📄 Чек файлы жоғарыда 👆",
		userId,
		state.Count,
		actualPrice,
		time.Now().Format("2006-01-02 15:04:05"))
	admins := []int64{h.cfg.AdminID, h.cfg.AdminID2}
	for i := 0; i < len(admins); i++ {
		admin := admins[i]
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			h.logger.Error("Failed to seek file to start", zap.Error(err))
		}

		_, errSendToAdmin := b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID: admin,
			Document: &models.InputFileUpload{
				Filename: fileName,
				Data:     f,
			},
			Caption: msgText,
		})
		if errSendToAdmin != nil {
			h.logger.Error("Failed to send file to admin", zap.Error(errSendToAdmin))
		}
	}

	kb := models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{
					Text:           "📲 Контактіні бөлісу",
					RequestContact: true,
				},
			},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
	successMessage := "✅ Чек PDF сәтті қабылданды! 🎉\n\n" +
		"📞 Сізбен кері байланысқа шығу үшін төмендегі\n" +
		"📲 Контактіні бөлісу түймесін 👇 міндетті басыңыз.\n\n"

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      update.Message.Chat.ID,
		Text:        successMessage,
		ReplyMarkup: kb,
	})
	if err != nil {
		h.logger.Warn("Failed to send confirmation message", zap.Error(err))
	}
}

func (h *Handler) ShareContactCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	userId := update.Message.From.ID

	if update.Message.Contact == nil {
		kb := models.ReplyKeyboardMarkup{
			Keyboard: [][]models.KeyboardButton{
				{
					{
						Text:           "📲 Контактіні бөлісу",
						RequestContact: true,
					},
				},
			},
			ResizeKeyboard:  true,
			OneTimeKeyboard: true,
		}
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      userId,
			Text:        "Cізбен кері байланысқа шығу үшін контактіні 📲 бөлісу түймесін басыңыз.",
			ReplyMarkup: kb,
		})
		if err != nil {
			h.logger.Warn("Failed to answer callback query", zap.Error(err))
			return
		}
		return
	}

	state, err := h.redisRepo.GetUserState(ctx, userId)
	if err != nil {
		h.logger.Error("Failed to get user state from Redis", zap.Error(err))
		state = &domain.UserState{
			State:  StateContact,
			Count:  1,
			IsPaid: true,
		}
	}
	if state != nil {
		state.Contact = update.Message.Contact.PhoneNumber
		if err := h.redisRepo.SaveUserState(ctx, userId, state); err != nil {
			h.logger.Error("Failed to save user state to Redis", zap.Error(err))
		}
	}
	// FIX: Use state data safely with nil checks
	userData := fmt.Sprintf("UserID: %d, State: %s, Count: %d, IsPaid: %t, Contact: %s",
		update.Message.From.ID,
		func() string {
			if state != nil {
				return state.State
			}
			return "unknown"
		}(),
		func() int {
			if state != nil {
				return state.Count
			}
			return 0
		}(),
		func() bool {
			if state != nil {
				return state.IsPaid
			}
			return false
		}(),
		func() string {
			if state != nil {
				return state.Contact
			}
			return ""
		}())
	h.logger.Info(userData)

	// FIXED: Use direct Mini App URL without bot username
	kb := models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "📍 Мекен-жайды енгізу",
					URL:  "t.me/zhad_parfume_bot/ZhadParfume", // Direct static URL
				},
			},
		},
	}

	_, errCheck := h.clientRepo.IsClientUnique(ctx, userId)
	if errCheck != nil {
		h.logger.Warn("Failed to check if client is paid", zap.Error(errCheck))
		return
	}

	entry := domain.ClientEntry{
		UserID:       userId,
		UserName:     update.Message.From.FirstName,
		Fio:          sql.NullString{},
		Contact:      state.Contact,
		Address:      sql.NullString{},
		DateRegister: sql.NullString{},
		DatePay:      time.Now().Format("2006-01-02 15:04:05"),
		Checks:       false,
	}

	order := domain.OrderEntry{
		UserID:       userId,
		Quantity:     state.Count,
		UserName:     update.Message.From.FirstName,
		Fio:          sql.NullString{},
		Address:      sql.NullString{},
		DateRegister: sql.NullString{},
		DatePay:      time.Now().Format("2006-01-02 15:04:05"),
		Checks:       false,
	}

	if err := h.clientRepo.InsertClient(ctx, entry); err != nil {
		h.logger.Warn("Failed to insert client", zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: h.cfg.AdminID,
			Text:   fmt.Sprintf("Error when save insert client, error: %s", err.Error()),
		})
	}

	if err := h.clientRepo.InsertOrder(ctx, order); err != nil {
		h.logger.Warn("Failed to insert order", zap.Error(err))
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: h.cfg.AdminID,
			Text:   fmt.Sprintf("Error when save insert order, error: %s", err.Error()),
		})
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: "✅ Контактіңіз сәтті алынды! 😊\n" +
			"Парфюм жинақты қай мекен-жайға жеткізу керек екенін көрсетіңіз. 🚚\n" +
			"⤵️ Мекен-жайыңызды енгізу үшін батырманы басыңыз👇",
		ReplyMarkup:    kb,
		ProtectContent: true,
	})
	if err != nil {
		h.logger.Warn("Failed to send confirmation message", zap.Error(err))
	}

	if err := h.redisRepo.DeleteUserState(ctx, userId); err != nil {
		h.logger.Error("Failed to delete user state from Redis", zap.Error(err))
	}
}

func (h *Handler) getOrCreateUserState(ctx context.Context, userID int64) *domain.UserState {
	state, err := h.redisRepo.GetUserState(ctx, userID)
	if err != nil {
		h.logger.Error("Redis error, using fallback state",
			zap.Error(err),
			zap.Int64("user_id", userID))

		// Return a safe default state
		return &domain.UserState{
			State:  StateStart,
			Count:  0,
			IsPaid: false,
		}
	}

	if state == nil {
		state = &domain.UserState{
			State:  StateStart,
			Count:  0,
			IsPaid: false,
		}

		// Try to save, but don't fail if Redis is down
		if err := h.redisRepo.SaveUserState(ctx, userID, state); err != nil {
			h.logger.Warn("Failed to save state to Redis, continuing with in-memory state",
				zap.Error(err))
		}
	}
	return state
}

// Fixed Handler methods - using repository methods instead of direct DB access

// ENHANCED GetUserAvailableQuantity with temporary selection awareness
func (h *Handler) GetUserAvailableQuantity(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telegramIDStr := r.URL.Query().Get("telegram_id")
	if telegramIDStr == "" {
		http.Error(w, "telegram_id parameter required", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram_id", http.StatusBadRequest)
		return
	}

	// Get user's orders
	orders, err := h.orderRepo.GetUnpaidOrdersByUser(telegramID)
	if err != nil {
		h.logger.Error("Error getting user orders", zap.Error(err))
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var totalQuantity int
	var temporaryQuantity int
	var orderDetails []map[string]interface{}
	var hasTemporarySelections bool

	for _, order := range orders {
		orderQuantity := 0
		if order.Quantity != nil {
			orderQuantity = *order.Quantity
		}

		// Parse existing perfume selections
		selectedPerfumes := []string{}
		usedQuantity := 0
		isTemporarySelection := false

		if order.Parfumes != "" {
			// Check if this is a temporary selection (has perfumes but no address)
			if order.Address != "" || order.Address == "" {
				isTemporarySelection = true
				hasTemporarySelections = true
			}

			parts := strings.Split(order.Parfumes, ",")
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					selectedPerfumes = append(selectedPerfumes, trimmed)
					// Extract quantity from format "name: quantity"
					if colonIndex := strings.Index(trimmed, ":"); colonIndex > 0 {
						if quantityStr := strings.TrimSpace(trimmed[colonIndex+1:]); quantityStr != "" {
							if qty, err := strconv.Atoi(quantityStr); err == nil {
								usedQuantity += qty
								if isTemporarySelection {
									temporaryQuantity += qty
								}
							}
						}
					}
				}
			}
		}

		availableInThisOrder := orderQuantity - usedQuantity
		if availableInThisOrder > 0 {
			totalQuantity += availableInThisOrder
		}

		orderDetails = append(orderDetails, map[string]interface{}{
			"id":                order.ID,
			"total_quantity":    orderQuantity,
			"used_quantity":     usedQuantity,
			"available":         availableInThisOrder,
			"selected_perfumes": selectedPerfumes,
			"is_temporary":      isTemporarySelection,
			"created_at":        order.CreatedAt,
		})
	}

	// FIXED: If we have temporary selections but backend shows 0 available,
	// restore access by adding back the temporary quantity
	effectiveAvailableQuantity := totalQuantity
	if totalQuantity == 0 && temporaryQuantity > 0 {
		effectiveAvailableQuantity = temporaryQuantity
		h.logger.Info("Restoring user access due to temporary selections",
			zap.Int64("telegram_id", telegramID),
			zap.Int("temporary_quantity", temporaryQuantity))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":                  true,
		"available_quantity":       effectiveAvailableQuantity,
		"original_available":       totalQuantity,
		"temporary_quantity":       temporaryQuantity,
		"has_temporary_selections": hasTemporarySelections,
		"access_restored":          totalQuantity == 0 && temporaryQuantity > 0,
		"orders":                   orderDetails,
	})
}

// ENHANCED SavePerfumeSelection with better temporary storage logic
func (h *Handler) SavePerfumeSelection(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TelegramID       int64                    `json:"telegram_id"`
		SelectedPerfumes []map[string]interface{} `json:"selected_perfumes"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.TelegramID == 0 {
		http.Error(w, "telegram_id required", http.StatusBadRequest)
		return
	}

	// Calculate total selected quantity
	totalSelected := 0
	for _, perfume := range req.SelectedPerfumes {
		if qty, ok := perfume["quantity"].(float64); ok {
			totalSelected += int(qty)
		}
	}

	// FIXED: Enhanced logic to handle both fresh selections and restored access
	var availableQuantity int
	var targetOrderID int64 = -1

	// First, get the user's original available quantity from unpaid orders
	originalAvailableQuantity, err := h.orderRepo.GetAvailableQuantityForUser(req.TelegramID)
	if err != nil {
		h.logger.Error("Error getting original available quantity", zap.Error(err))
		http.Error(w, "Error checking available quantity", http.StatusInternalServerError)
		return
	}

	// Check if user had temporary selections that we need to account for
	orders, err := h.orderRepo.GetUnpaidOrdersByUser(req.TelegramID)
	if err != nil {
		h.logger.Error("Error finding orders", zap.Error(err))
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Calculate previously used quantity from temporary selections
	var previousTempQuantity int
	for _, order := range orders {
		if order.Parfumes != "" && order.Address == "" {
			// This is a temporary selection - count its quantity
			parts := strings.Split(order.Parfumes, ",")
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					if colonIndex := strings.Index(trimmed, ":"); colonIndex > 0 {
						if quantityStr := strings.TrimSpace(trimmed[colonIndex+1:]); quantityStr != "" {
							if qty, err := strconv.Atoi(quantityStr); err == nil {
								previousTempQuantity += qty
								if targetOrderID == -1 {
									targetOrderID = order.ID // Use this order for updating
								}
							}
						}
					}
				}
			}
		}
	}

	// FIXED: If no temporary selections exist, find a fresh order to use
	if targetOrderID == -1 {
		for _, order := range orders {
			if order.Quantity == nil {
				continue
			}
			orderQuantity := *order.Quantity

			// Calculate used quantity in this order
			usedQuantity := 0
			if order.Parfumes != "" {
				parts := strings.Split(order.Parfumes, ",")
				for _, part := range parts {
					if trimmed := strings.TrimSpace(part); trimmed != "" {
						if colonIndex := strings.Index(trimmed, ":"); colonIndex > 0 {
							if quantityStr := strings.TrimSpace(trimmed[colonIndex+1:]); quantityStr != "" {
								if qty, err := strconv.Atoi(quantityStr); err == nil {
									usedQuantity += qty
								}
							}
						}
					}
				}
			}

			availableInThisOrder := orderQuantity - usedQuantity
			if availableInThisOrder > 0 {
				targetOrderID = order.ID
				break
			}
		}
	}

	// Calculate effective available quantity
	if previousTempQuantity > 0 {
		// User had temporary selections - restore their effective available quantity
		availableQuantity = previousTempQuantity
		h.logger.Info("Restoring user access with temporary quantity",
			zap.Int64("telegram_id", req.TelegramID),
			zap.Int("previous_temp_quantity", previousTempQuantity),
			zap.Int("original_available", originalAvailableQuantity))
	} else {
		// Fresh selection - use original available quantity
		availableQuantity = originalAvailableQuantity
	}

	// Validate against effective available quantity
	if totalSelected > availableQuantity {
		http.Error(w, fmt.Sprintf("Not enough quantity available. You have %d, trying to select %d",
			availableQuantity, totalSelected), http.StatusBadRequest)
		return
	}

	if targetOrderID == -1 {
		http.Error(w, "No available orders found", http.StatusBadRequest)
		return
	}

	// Build perfume selection string (format: "name: quantity, name: quantity")
	var parfumeSelections []string
	for _, perfume := range req.SelectedPerfumes {
		name, nameOk := perfume["name"].(string)
		qty, qtyOk := perfume["quantity"].(float64)
		if nameOk && qtyOk && qty > 0 {
			parfumeSelections = append(parfumeSelections, fmt.Sprintf("%s: %d", name, int(qty)))
		}
	}

	parfumeString := strings.Join(parfumeSelections, ", ")

	// Update the order with perfume selection (this creates temporary selection)
	err = h.orderRepo.UpdatePerfumeSelection(targetOrderID, parfumeString)
	if err != nil {
		h.logger.Error("Error updating order with perfumes", zap.Error(err))
		http.Error(w, "Error saving selection", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Perfume selection saved (temporary)",
		zap.Int64("telegram_id", req.TelegramID),
		zap.Int64("order_id", targetOrderID),
		zap.String("perfumes", parfumeString),
		zap.Bool("is_restored_access", previousTempQuantity > 0))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         "Perfume selection saved successfully",
		"order_id":        targetOrderID,
		"perfumes":        parfumeString,
		"is_temporary":    true,
		"restored_access": previousTempQuantity > 0,
	})
}

// UpdateOrderWithClientInfo updates order with client information after address form
func (h *Handler) UpdateOrderWithClientInfo(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	telegramIDStr := r.FormValue("telegram_id")
	fio := r.FormValue("fio")
	contact := r.FormValue("contact")
	address := r.FormValue("address")
	latitudeStr := r.FormValue("latitude")
	longitudeStr := r.FormValue("longitude")

	if telegramIDStr == "" || fio == "" || contact == "" || address == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram_id", http.StatusBadRequest)
		return
	}

	// Parse coordinates if provided
	var latitude, longitude *float64
	if latitudeStr != "" {
		if lat, err := strconv.ParseFloat(latitudeStr, 64); err == nil {
			latitude = &lat
		}
	}
	if longitudeStr != "" {
		if lng, err := strconv.ParseFloat(longitudeStr, 64); err == nil {
			longitude = &lng
		}
	}

	// Find the order with perfume selection using repository method
	order, err := h.orderRepo.GetOrderWithPerfumeSelection(telegramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "No perfume selection found. Please select perfumes first", http.StatusBadRequest)
		} else {
			h.logger.Error("Error finding order", zap.Error(err))
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}

	// Update the order with client information including coordinates
	err = h.orderRepo.UpdateClientInfoWithCoordinates(order.ID, fio, contact, address)
	if err != nil {
		h.logger.Error("Error updating order with client info", zap.Error(err))
		http.Error(w, "Error saving client information", http.StatusInternalServerError)
		return
	}

	// Send success message to user via Telegram
	if h.bot != nil {
		go h.sendOrderConfirmationMessage(telegramID, order.ID, order.UserName, order.Parfumes, fio, contact, address)
	}

	h.logger.Info("Order updated with client info",
		zap.Int64("telegram_id", telegramID),
		zap.Int64("order_id", order.ID),
		zap.String("fio", fio),
		zap.String("contact", contact),
		zap.String("address", address),
		zap.Any("latitude", latitude),
		zap.Any("longitude", longitude))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Order completed successfully",
		"order_id": order.ID,
	})
}

// Send order confirmation message to Telegram
func (h *Handler) sendOrderConfirmationMessage(telegramID, orderID int64, userName, parfumes, fio, contact, address string) {
	if h.bot == nil {
		h.logger.Error("Bot not initialized")
		return
	}

	// Build message
	var messageText strings.Builder
	messageText.WriteString("✅ Тапсырыс сәтті рәсімделді!\n\n")
	messageText.WriteString(fmt.Sprintf("📦 Тапсырыс №: %d\n", orderID))
	messageText.WriteString(fmt.Sprintf("👤 Клиент: %s\n", fio))
	messageText.WriteString(fmt.Sprintf("📱 Телефон: %s\n", contact))
	messageText.WriteString(fmt.Sprintf("📍 Мекенжай: %s\n\n", address))
	messageText.WriteString("🌸 Таңдалған парфюмдер:\n")
	messageText.WriteString(fmt.Sprintf("_%s_\n\n", parfumes))
	messageText.WriteString("🚚 Жеткізу туралы ақпарат:\n")
	messageText.WriteString("Біздің менеджер сізбен 48 сағат ішінде байланысады.\n\n")
	messageText.WriteString("Рахмет! 💝")

	// Send message to user
	_, err := h.bot.SendMessage(h.ctx, &bot.SendMessageParams{
		ChatID: telegramID,
		Text:   messageText.String(),
	})

	if err != nil {
		h.logger.Error("Failed to send confirmation message to user",
			zap.Error(err),
			zap.Int64("telegram_id", telegramID),
			zap.Int64("order_id", orderID))
	} else {
		h.logger.Info("Order confirmation sent to user successfully",
			zap.Int64("telegram_id", telegramID),
			zap.Int64("order_id", orderID))
	}

	// Send notification to admin
	adminMessage := fmt.Sprintf(
		"📋 Жаңа тапсырыс!\n\n"+
			"🆔 Тапсырыс: %d\n"+
			"👤 Клиент: %s (@%s)\n"+
			"📱 Телефон: %s\n"+
			"📍 Мекенжай: %s\n"+
			"🌸 Парфюмдер: %s\n"+
			"⏰ Уақыт: %s",
		orderID, fio, userName, contact, address, parfumes,
		time.Now().Format("2006-01-02 15:04:05"))

	admins := []int64{h.cfg.AdminID, h.cfg.AdminID2}
	for _, adminID := range admins {
		if adminID != 0 {
			_, err := h.bot.SendMessage(h.ctx, &bot.SendMessageParams{
				ChatID: adminID,
				Text:   adminMessage,
			})
			if err != nil {
				h.logger.Error("Failed to send admin notification",
					zap.Error(err),
					zap.Int64("admin_id", adminID))
			}
		}
	}
}

// GetUserTemporarySelections retrieves user's temporary perfume selections
func (h *Handler) GetUserTemporarySelections(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telegramIDStr := r.URL.Query().Get("telegram_id")
	if telegramIDStr == "" {
		http.Error(w, "telegram_id parameter required", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram_id", http.StatusBadRequest)
		return
	}

	// Get orders with perfume selections that haven't been finalized (no address yet)
	orders, err := h.orderRepo.GetUnpaidOrdersByUser(telegramID)
	if err != nil {
		h.logger.Error("Error getting user orders for temp selections", zap.Error(err))
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var temporarySelections []map[string]interface{}
	var totalTempQuantity int

	for _, order := range orders {
		// Check if this order has perfume selections but no address (meaning it's temporary)
		if order.Parfumes != "" && (order.Address == "" || order.Address == "") {
			// Parse the perfume selections
			parts := strings.Split(order.Parfumes, ",")
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					// Extract name and quantity from format "name: quantity"
					if colonIndex := strings.Index(trimmed, ":"); colonIndex > 0 {
						name := strings.TrimSpace(trimmed[:colonIndex])
						quantityStr := strings.TrimSpace(trimmed[colonIndex+1:])
						if quantity, err := strconv.Atoi(quantityStr); err == nil && quantity > 0 {
							// Try to find the perfume ID by name
							perfumeID := h.findPerfumeIDByName(name)
							if perfumeID != "" {
								temporarySelections = append(temporarySelections, map[string]interface{}{
									"id":       perfumeID,
									"name":     name,
									"quantity": quantity,
								})
								totalTempQuantity += quantity
							}
						}
					}
				}
			}
		}
	}

	h.logger.Info("Retrieved temporary selections",
		zap.Int64("telegram_id", telegramID),
		zap.Int("total_temp_quantity", totalTempQuantity),
		zap.Int("selection_count", len(temporarySelections)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":             true,
		"selections":          temporarySelections,
		"total_quantity":      totalTempQuantity,
		"has_temp_selections": len(temporarySelections) > 0,
	})
}

// Helper function to find perfume ID by name
func (h *Handler) findPerfumeIDByName(name string) string {
	perfumes, err := h.parfumeRepo.GetAll()
	if err != nil {
		h.logger.Error("Error getting perfumes for name lookup", zap.Error(err))
		return ""
	}

	for _, perfume := range perfumes {
		if perfume.NameParfume == name {
			return perfume.Id
		}
	}
	return ""
}

// SetBot sets the bot instance for the handler
func (h *Handler) SetBot(b *bot.Bot) {
	h.bot = b
}

// Update your StartWebServer method to include prize routes
func (h *Handler) StartWebServer(ctx context.Context, b *bot.Bot) {
	h.SetBot(b)

	// Create required directories
	directories := []string{"./static", "./files", "./payments", "./photo"}
	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.logger.Error("Failed to create directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	// CORS Middleware
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("/static/", corsMiddleware(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/")))))
	mux.Handle("/files/", corsMiddleware(http.StripPrefix("/files/", http.FileServer(http.Dir("./files/")))))
	mux.Handle("/photo/", corsMiddleware(h.createPhotoHandler()))

	// Main routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/parfume.html"
		http.ServeFile(w, r, path)
	})

	mux.HandleFunc("/parfume", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/parfume.html"
		http.ServeFile(w, r, path)
	})

	mux.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/client-form.html"
		http.ServeFile(w, r, path)
	})

	// NEW: Prize wheel route
	mux.HandleFunc("/prize", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/prize.html"
		http.ServeFile(w, r, path)
	})

	// Admin routes
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/admin-parfume.html"
		http.ServeFile(w, r, path)
	})

	// API endpoints
	mux.HandleFunc("/api/parfumes", h.handleGetPerfumes)
	mux.HandleFunc("/api/parfume/", h.handleGetPerfume)
	mux.HandleFunc("/api/add-parfume", h.handleAddPerfume)
	mux.HandleFunc("/api/update-parfume/", h.handleUpdatePerfume)
	mux.HandleFunc("/api/delete-parfume/", h.handleDeletePerfume)
	mux.HandleFunc("/api/search-parfumes", h.handleSearchPerfumes)

	// Perfume selection service
	mux.HandleFunc("/api/user/available-quantity", h.GetUserAvailableQuantity)
	mux.HandleFunc("/api/user/temp-selections", h.GetUserTemporarySelections)
	mux.HandleFunc("/api/user/save-perfume-selection", h.SavePerfumeSelection)
	mux.HandleFunc("/api/order/complete", h.UpdateOrderWithClientInfo)

	// NEW: Prize wheel endpoints
	mux.HandleFunc("/api/prize/eligibility", h.CheckSpinEligibility)
	mux.HandleFunc("/api/prize/spin", h.SpinWheel)
	mux.HandleFunc("/api/prize/complete", h.CompletePrizeOrder)

	// Existing endpoints
	mux.HandleFunc("/api/orders", h.handleGetOrders)
	mux.HandleFunc("/api/order/", h.handleGetOrder)

	// Health check
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
			"service":   "zhad-perfume-api-with-prizes",
			"version":   "4.0.0-prize-wheel",
		})
	})

	h.logger.Info("Starting web server with prize wheel functionality", zap.String("port", h.cfg.Port))

	if err := http.ListenAndServe(h.cfg.Port, mux); err != nil {
		h.logger.Fatal("Failed to start web server", zap.Error(err))
	}
}


// Create photo handler (helper method)
func (h *Handler) createPhotoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/photo/")
		if filename == "" {
			h.logger.Warn("Empty photo filename requested", zap.String("url", r.URL.Path))
			http.NotFound(w, r)
			return
		}

		filePath := filepath.Join("./photo", filename)

		h.logger.Info("Photo request",
			zap.String("url", r.URL.Path),
			zap.String("filename", filename),
			zap.String("filepath", filePath))

		fileInfo, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			h.logger.Warn("Photo file not found", zap.String("filepath", filePath))
			http.NotFound(w, r)
			return
		} else if err != nil {
			h.logger.Error("Error accessing photo file", zap.Error(err))
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
			return
		}

		h.logger.Info("Photo file found",
			zap.String("filepath", filePath),
			zap.Int64("size", fileInfo.Size()))

		w.Header().Set("Cache-Control", "public, max-age=86400")

		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".jpg", ".jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		case ".gif":
			w.Header().Set("Content-Type", "image/gif")
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
		case ".svg":
			w.Header().Set("Content-Type", "image/svg+xml")
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
		}

		http.ServeFile(w, r, filePath)
		h.logger.Info("Photo served successfully", zap.String("filename", filename))
	})
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

	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

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

	if sex != "Male" && sex != "Female" && sex != "Unisex" {
		http.Error(w, "Invalid sex value", http.StatusBadRequest)
		return
	}

	var photoPath string
	file, fileHeader, err := r.FormFile("photo")
	if err == nil {
		defer file.Close()

		ext := filepath.Ext(fileHeader.Filename)
		filename := uuid.New().String() + ext
		photoPath = filename

		dst, err := os.Create(filepath.Join("./photo", filename))
		if err != nil {
			h.logger.Error("Error creating photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			h.logger.Error("Error copying photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
	}

	perfume := &repository.Product{
		NameParfume: name,
		Sex:         sex,
		Description: description,
		Price:       price,
		PhotoPath:   photoPath,
	}

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

	path := strings.TrimPrefix(r.URL.Path, "/api/update-parfume/")
	if path == "" {
		http.Error(w, "Perfume ID required", http.StatusBadRequest)
		return
	}

	existingPerfume, err := h.parfumeRepo.GetByID(path)
	if err != nil {
		h.logger.Error("Error getting perfume for update", zap.Error(err))
		http.Error(w, "Perfume not found", http.StatusNotFound)
		return
	}

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

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

	if sex != "Male" && sex != "Female" && sex != "Unisex" {
		http.Error(w, "Invalid sex value", http.StatusBadRequest)
		return
	}

	photoPath := existingPerfume.PhotoPath
	file, fileHeader, err := r.FormFile("photo")
	if err == nil {
		defer file.Close()

		if existingPerfume.PhotoPath != "" {
			oldPhotoPath := filepath.Join("./photo", existingPerfume.PhotoPath)
			os.Remove(oldPhotoPath)
		}

		ext := filepath.Ext(fileHeader.Filename)
		filename := uuid.New().String() + ext
		photoPath = filename

		dst, err := os.Create(filepath.Join("./photo", filename))
		if err != nil {
			h.logger.Error("Error creating photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			h.logger.Error("Error copying photo file", zap.Error(err))
			http.Error(w, "Error uploading photo", http.StatusInternalServerError)
			return
		}
	}

	updatedPerfume := &repository.Product{
		Id:          existingPerfume.Id,
		NameParfume: name,
		Sex:         sex,
		Description: description,
		Price:       price,
		PhotoPath:   photoPath,
	}

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

	path := strings.TrimPrefix(r.URL.Path, "/api/delete-parfume/")
	if path == "" {
		http.Error(w, "Perfume ID required", http.StatusBadRequest)
		return
	}

	perfume, err := h.parfumeRepo.GetByID(path)
	if err != nil {
		h.logger.Error("Error getting perfume for deletion", zap.Error(err))
		http.Error(w, "Perfume not found", http.StatusNotFound)
		return
	}

	err = h.parfumeRepo.Delete(path)
	if err != nil {
		h.logger.Error("Error deleting perfume", zap.Error(err))
		http.Error(w, "Error deleting perfume", http.StatusInternalServerError)
		return
	}

	if perfume.PhotoPath != "" {
		photoPath := filepath.Join("./photo", perfume.PhotoPath)
		err := os.Remove(photoPath)
		if err != nil {
			h.logger.Warn("Error deleting photo file", zap.Error(err))
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
		perfumes, err = h.parfumeRepo.AdvancedSearch(query, sex, minPrice, maxPrice)
	} else {
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

// Get client data by telegram ID
func (h *Handler) handleGetClientData(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		TelegramID int64 `json:"telegram_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if requestData.TelegramID == 0 {
		http.Error(w, "Telegram ID required", http.StatusBadRequest)
		return
	}

	client, err := h.clientRepo.GetByTelegramID(requestData.TelegramID)
	if err != nil {
		// Client not found is not an error, just return empty
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Client not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"client":  client,
	})
}

// Save client data
func (h *Handler) handleSaveClient(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	telegramIDStr := r.FormValue("telegram_id")
	fio := r.FormValue("fio")
	contact := r.FormValue("contact")
	address := r.FormValue("address")
	latitude := r.FormValue("latitude")
	longitude := r.FormValue("longitude")

	if telegramIDStr == "" || fio == "" || contact == "" || address == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram ID", http.StatusBadRequest)
		return
	}

	client := &domain.Client{
		TelegramID: telegramID,
		FIO:        fio,
		Contact:    contact,
		Address:    address,
		Latitude:   latitude,
		Longitude:  longitude,
	}

	err = h.clientRepo.SaveOrUpdate(client)
	if err != nil {
		h.logger.Error("Error saving client", zap.Error(err))
		http.Error(w, "Error saving client", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Client saved successfully",
	})
}

// Place order
func (h *Handler) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	telegramIDStr := r.FormValue("telegram_id")
	fio := r.FormValue("fio")
	contact := r.FormValue("contact")
	address := r.FormValue("address")
	latitude := r.FormValue("latitude")
	longitude := r.FormValue("longitude")
	cartDataStr := r.FormValue("cart_data")
	totalAmountStr := r.FormValue("total_amount")

	if telegramIDStr == "" || fio == "" || contact == "" || address == "" || cartDataStr == "" || totalAmountStr == "" {
		http.Error(w, "Required fields missing", http.StatusBadRequest)
		return
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid telegram ID", http.StatusBadRequest)
		return
	}

	totalAmount, err := strconv.Atoi(totalAmountStr)
	if err != nil {
		http.Error(w, "Invalid total amount", http.StatusBadRequest)
		return
	}

	// Parse cart data
	var cartItems []CartItem
	err = json.Unmarshal([]byte(cartDataStr), &cartItems)
	if err != nil {
		http.Error(w, "Invalid cart data", http.StatusBadRequest)
		return
	}

	// Save/update client first
	client := &domain.Client{
		TelegramID: telegramID,
		FIO:        fio,
		Contact:    contact,
		Address:    address,
		Latitude:   latitude,
		Longitude:  longitude,
	}

	err = h.clientRepo.SaveOrUpdate(client)
	if err != nil {
		h.logger.Error("Error saving client", zap.Error(err))
		http.Error(w, "Error saving client", http.StatusInternalServerError)
		return
	}

	// Get saved client to get ID
	savedClient, err := h.clientRepo.GetByTelegramID(telegramID)
	if err != nil {
		h.logger.Error("Error getting saved client", zap.Error(err))
		http.Error(w, "Error processing order", http.StatusInternalServerError)
		return
	}

	// Generate payment link (you can customize this)
	orderID := fmt.Sprintf("LMN-%d-%d", telegramID, time.Now().Unix())
	paymentLink := fmt.Sprintf("https://pay.kaspi.kz/pay/%s?amount=%d", orderID, totalAmount)

	// Create order
	order := &domain.Order{
		ID:     telegramID,
		IDUser: savedClient.ID,
	}

	err = h.orderRepo.Create(order)
	if err != nil {
		h.logger.Error("Error creating order", zap.Error(err))
		http.Error(w, "Error creating order", http.StatusInternalServerError)
		return
	}

	// Send order confirmation to Telegram bot
	go h.sendOrderConfirmation(telegramID, cartItems, totalAmount, paymentLink, orderID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"message":      "Order placed successfully",
		"order_id":     orderID,
		"payment_link": paymentLink,
	})
}

// Send order confirmation via Telegram
func (h *Handler) sendOrderConfirmation(telegramID int64, cartItems []CartItem, totalAmount int, paymentLink, orderID string) {
	if h.bot == nil {
		h.logger.Error("Bot not initialized")
		return
	}

	// Build order message
	var orderText strings.Builder
	orderText.WriteString("🌟 *Lumen Парфюмерия* - Тапсырыс растауы\n\n")
	orderText.WriteString(fmt.Sprintf("📦 *Тапсырыс №:* `%s`\n\n", orderID))
	orderText.WriteString("🛒 *Сіздің тапсырысыңыз:*\n")

	for _, item := range cartItems {
		orderText.WriteString(fmt.Sprintf("• %s\n", item.Name))
		orderText.WriteString(fmt.Sprintf("  Саны: %d дана\n", item.Quantity))
		orderText.WriteString(fmt.Sprintf("  Бағасы: %s₸\n", formatPrice(item.Price*item.Quantity)))
		orderText.WriteString("\n")
	}

	orderText.WriteString("━━━━━━━━━━━━━━━━━━\n")
	orderText.WriteString(fmt.Sprintf("💰 *Жалпы сома: %s₸*\n\n", formatPrice(totalAmount)))
	orderText.WriteString("Төлеу үшін төмендегі түймені басыңыз 👇")

	// Create payment keyboard
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text: "💳 Төлеу жасау",
					URL:  "",
				},
			},
			{
				{
					Text: "📞 Қолдау қызметі",
					URL:  "https://t.me/lumen_support",
				},
			},
		},
	}

	// Send message
	_, err := h.bot.SendMessage(h.ctx, &bot.SendMessageParams{
		ChatID:      telegramID,
		Text:        orderText.String(),
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		h.logger.Error("Failed to send order confirmation",
			zap.Error(err),
			zap.Int64("telegram_id", telegramID),
			zap.String("order_id", orderID))
	} else {
		h.logger.Info("Order confirmation sent successfully",
			zap.Int64("telegram_id", telegramID),
			zap.String("order_id", orderID))
	}
}

// Get orders (admin endpoint)
func (h *Handler) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orders, err := h.orderRepo.GetAll()
	if err != nil {
		h.logger.Error("Error getting orders", zap.Error(err))
		http.Error(w, "Error getting orders", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// Get single order
func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w)
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/order/")
	if path == "" {
		http.Error(w, "Order ID required", http.StatusBadRequest)
		return
	}

	orderID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, err := h.orderRepo.GetByID(orderID)
	if err != nil {
		h.logger.Error("Error getting order", zap.Error(err))
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// Helper functions
func (h *Handler) setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func formatPrice(price int) string {
	// Add thousand separators
	priceStr := strconv.Itoa(price)
	if len(priceStr) <= 3 {
		return priceStr
	}

	var result strings.Builder
	for i, digit := range priceStr {
		if i > 0 && (len(priceStr)-i)%3 == 0 {
			result.WriteString(" ")
		}
		result.WriteRune(digit)
	}

	return result.String()
}

func stringPtr(s string) *string {
	return &s
}

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

func NewHandler(cfg *config.Config, zapLogger *zap.Logger, ctx context.Context, db *sql.DB) *Handler {
	h := &Handler{
		cfg:         cfg,
		logger:      zapLogger,
		ctx:         ctx,
		parfumeRepo: repository.NewParfumeRepository(db),
		clientRepo:  repository.NewClientRepository(db),
		orderRepo:   repository.NewOrderRepository(db),
	}

	return h
}

func (h *Handler) StartHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	promoText := "24900 теңгеге парфюм жиынтық сатып алыңыз және сыйлықтар ұтып алыңыз!"

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
	_, err := b.SendVideo(ctx, &bot.SendVideoParams{
		ChatID:         update.Message.Chat.ID,
		Video:          &models.InputFileString{Data: h.cfg.StartVideoId},
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

	switch userState.State {
	case StateStart:
		h.StartHandler(ctx, b, update)
		return
	case StateDefault:
		h.DefaultHandler(ctx, b, update)
		return
	case StateCount:

	}
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Welcome to Parfum Bot!",
	})
	if err != nil {
		h.logger.Error("failed to send message", zap.Error(err))
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
			num := 5*j + 1
			row[j] = models.InlineKeyboardButton{
				Text:         strconv.Itoa(num),
				CallbackData: fmt.Sprintf("count_%d", num),
			}
		}
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
		return
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
	if !ok {
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

	willBePrice := []int{18000, 18100, 18200, 18300, 18400, 18500, 18600, 18700, 18800, 18990, 18900, 19000, 19500, 19800, 19890, 19900, 19990, 20000, 21000, 29000}
	for i := 0; i < len(willBePrice); i++ {
		if actualPrice == willBePrice[i] {
			actualPrice = 18900
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

	// Just increase the total sum
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
					URL:  "https://t.me/meilly_cosmetics_bot/MeiLyCosmetics", // Direct static URL
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

	fmt.Println("Count: ", state.Count)

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

	_, err = b.SendVideo(ctx, &bot.SendVideoParams{
		ChatID: update.Message.Chat.ID,
		Video: &models.InputFileString{
			Data: h.cfg.InstructorVideoId,
		},
		Caption: "✅ Контактіңіз сәтті алынды! 😊\n" +
			"Косметикалық жинақты қай мекен-жайға жеткізу керек екенін көрсетіңіз. 🚚\n" +
			"⤵️ Мекен-жайыңызды енгізу үшін батырманы басыңыз👇\nТолығырақ 📹 видео инструкцияда",
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
			State:  "",
			Count:  0,
			IsPaid: false,
		}
	}

	if state == nil {
		state = &domain.UserState{
			State:  "",
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

	// CORS Middleware
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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
	mux.Handle("/photo/", corsMiddleware(http.StripPrefix("/photo/", http.FileServer(http.Dir("./photo/")))))

	// Page routes
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

	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		path := "./static/admin-parfume.html"
		http.ServeFile(w, r, path)
	})

	mux.HandleFunc("/admin/add-perfume", func(w http.ResponseWriter, r *http.Request) {
		h.setCORSHeaders(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		path := "./static/admin-add-parfume.html"
		http.ServeFile(w, r, path)
	})

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

	// Client API endpoints
	mux.HandleFunc("/api/client/data", h.handleGetClientData)
	mux.HandleFunc("/api/client/save", h.handleSaveClient)

	// Order API endpoints
	mux.HandleFunc("/api/order/place", h.handlePlaceOrder)
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
			"service":   "lumen-perfume-api",
			"version":   "3.0.0-lumen",
		})
	})

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
		TelegramID:  telegramID,
		ClientID:    savedClient.ID,
		CartData:    cartDataStr,
		TotalAmount: totalAmount,
		Status:      "pending",
		PaymentLink: paymentLink,
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

package main

import (
	"log"
	"os"

	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/modes/classic"
	"chess-monolith/internal/matchmaking"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// initDB настраивает и возвращает подключение к PostgreSQL
func initDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("Connected to database")
	return db
}

// initGameRegistry собирает реестр правил и регистрирует все доступные режимы
func initGameRegistry() *core.Registry {
	registry := core.NewRegistry()
	classic.Register(registry) // Добавляем классические шахматы
	// TODO В будущем добавить: chess960.Register(registry) и т.д.
	return registry
}

// SetupRouter настраивает middlewares и эндпоинты
func SetupRouter(userHandler *users.Handler, hub *ws.Hub, userRepo users.Repository, jwtSecret string, qm ws.QueueManager, gameRepo game.Repository) *gin.Engine {
	router := gin.Default()

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "pong"})
	})

	router.GET("/ws", ws.ServeWS(hub, userRepo, jwtSecret, qm))

	userHandler.SetupRoutes(router)

	return router
}

func main() {
	// 1. Конфигурация
	if err := godotenv.Load("configs/.env"); err != nil {
		log.Println("Warning: Error loading .env file (fallback to system env)")
	}

	dsn := os.Getenv("DB_DSN")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtSecret := os.Getenv("JWT_SECRET")

	// 2. Инициализация инфраструктуры
	db := initDB(dsn)
	registry := initGameRegistry()

	// 3. Слой репозиториев (работа с БД)
	userRepo := users.NewRepository(db)
	gameRepo := game.NewRepository(db)

	// 4. Фоновые процессы (горутины)
	matchmaker := matchmaking.NewMatchmaker(registry, gameRepo, userRepo)
	go matchmaker.Run()

	hub := ws.NewHub()
	go hub.Run()

	// 5. Бизнес-логика (сервисы и хендлеры)
	userService := users.NewService(userRepo, jwtSecret)
	userHandler := users.NewHandler(userService)

	// 6. Роутинг и запуск сервера
	router := SetupRouter(userHandler, hub, userRepo, jwtSecret, matchmaker, gameRepo)

	log.Println("Starting server on port " + port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

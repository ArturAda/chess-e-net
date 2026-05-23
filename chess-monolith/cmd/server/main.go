package main

import (
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupRouter(userHandler *users.Handler, hub *ws.Hub, userRepo users.Repository, jwtSecret string) *gin.Engine {
	router := gin.Default()

	config := cors.DefaultConfig()

	config.AllowAllOrigins = true // zДля MVP сойдет, потом заменишь на конкретный домен
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "pong"})
	})

	router.GET("/ws", ws.ServeWS(hub, userRepo, jwtSecret))

	userHandler.SetupRoutes(router)

	return router
}

func main() {
	if err := godotenv.Load("configs/.env"); err != nil {
		log.Println("Error loading .env file")
	}

	dsn := os.Getenv("DB_DSN")
	port := os.Getenv("PORT")
	jwtSecret := os.Getenv("JWT_SECRET")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("Connected to database")

	if err := db.AutoMigrate(&users.User{}); err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}
	log.Println("Created users table")

	hub := ws.NewHub()
	go hub.Run()

	userRepo := users.NewRepository(db)
	userService := users.NewService(userRepo, jwtSecret)
	userHandler := users.NewHandler(userService)

	router := SetupRouter(userHandler, hub, userRepo, jwtSecret)

	log.Println("Starting server on port " + port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

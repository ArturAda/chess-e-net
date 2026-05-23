package main

import (
	"chess-monolith/internal/users"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func SetupRouter() *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	router.GET("/api/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "pong"})
	})

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

	userRepo := users.NewRepository(db)
	userService := users.NewService(userRepo, jwtSecret)
	userHandler := users.NewHandler(userService)

	router := SetupRouter()

	userHandler.SetupRoutes(router)

	log.Println("Starting server on port " + port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

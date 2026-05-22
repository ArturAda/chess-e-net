// Файл: cmd/server/main.go
package main

import (
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
	log.Printf("DEBUG: DSN is %s", dsn)

	port := os.Getenv("PORT")

	_, err := gorm.Open(postgres.Open(dsn), &gorm.Config{}) //11
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	log.Println("Connected to database")

	// TODO (инициализация репозиториев и сервисов)

	router := SetupRouter()

	log.Println("Starting server on port " + port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

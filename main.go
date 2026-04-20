package main

import (
	"log"
	"queueflow/config"
	"queueflow/controllers"
	"queueflow/middleware"
	"queueflow/repositories"
	"queueflow/services"
	"queueflow/websocket"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberwebsocket "github.com/gofiber/websocket/v2"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Connect to database
	db, err := config.ConnectDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := config.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create default users (admin/admin123, user1/user123)
	passwordHash, _ := services.HashPassword("password123")
	if err := config.CreateDefaultUsers(db, passwordHash); err != nil {
		log.Printf("Warning: Failed to create default users: %v", err)
	}

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	queueRepo := repositories.NewQueueRepository(db)

	// Initialize WebSocket manager
	wsManager := websocket.NewManager()
	go wsManager.Run()

	// Initialize services
	authService := services.NewAuthService(userRepo, cfg.JWTSecret)
	queueService := services.NewQueueService(queueRepo, wsManager)

	// Initialize controllers
	authController := controllers.NewAuthController(authService)
	queueController := controllers.NewQueueController(queueService)
	wsController := controllers.NewWebSocketController(wsManager)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "QueueFlow API v1.0",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "QueueFlow API is running",
		})
	})

	// Public routes
	app.Post("/auth/login", authController.Login)

	// WebSocket route (requires authentication)
	app.Use("/ws", func(c *fiber.Ctx) error {
		if fiberwebsocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", middleware.AuthMiddleware(cfg.JWTSecret), wsController.HandleConnection)

	// Protected user routes
	queue := app.Group("/queue", middleware.AuthMiddleware(cfg.JWTSecret))
	queue.Post("/join", queueController.JoinQueue)
	queue.Post("/leave", queueController.LeaveQueue)
	queue.Post("/confirm", queueController.ConfirmTurn)
	queue.Get("/status", queueController.GetQueueStatus)
	queue.Get("/list", queueController.GetQueueList)

	// Protected admin routes
	admin := app.Group("/admin", middleware.AuthMiddleware(cfg.JWTSecret), middleware.AdminMiddleware())
	admin.Post("/next", queueController.CallNext)
	admin.Post("/remove-user/:user_id", queueController.RemoveUser)
	admin.Post("/pause", queueController.PauseQueue)
	admin.Post("/resume", queueController.ResumeQueue)

	// Start server
	port := ":" + cfg.Port
	log.Printf("Starting QueueFlow server on port %s", cfg.Port)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", port)
	log.Fatal(app.Listen("0.0.0.0" + port))
}

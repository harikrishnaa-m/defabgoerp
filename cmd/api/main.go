package main

import (
	"log"
	"os"

	"defab-erp/internal/auth"
	"defab-erp/internal/core/db"

	"defab-erp/internal/core/model"
	"defab-erp/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("⚠ .env file not found, using system ENV")
	}
	// 1. DB
	database := db.Connect()
	defer database.Close()

	// 2. Stores (Data Layer)
	authStore := auth.NewStore(database)
	// productStore := product.NewStore(database)

	// 3. Handlers (HTTP Layer)
	authHandler := auth.NewHandler(authStore)
	// productHandler := product.NewHandler(productStore)

	// 4. Fiber
	app := fiber.New()

	app.Use(logger.New())
	app.Use(recover.New())


	api := app.Group("/api")

	authRoutes := api.Group("/auth")
	authRoutes.Post("/register", authHandler.Register)
	 authRoutes.Post("/login", authHandler.Login)

	// 5. Routes
	// app.Post("/api/users", authHandler.Register)

	// Quick test route for products
	api.Get("/products/test", func(c *fiber.Ctx) error {
		// Just to prove store works
		return c.SendString("Product Store Connected")
	})

	// ---------- PROTECTED ----------
protected := api.Group("", middleware.JWTProtected(authStore))

protected.Get("/me", func(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	return c.JSON(user)
})

protected.Get("/admin-only",
	middleware.RequireRole("SuperAdmin"),
	func(c *fiber.Ctx) error {
		return c.SendString("Hello SuperAdmin")
	},
)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))
}

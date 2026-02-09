package main

import (
	"log"
	"os"

	"defab-erp/internal/auth"
	"defab-erp/internal/core/db"
	"defab-erp/internal/warehouse"

	"defab-erp/internal/core/model"
	"defab-erp/internal/middleware"

	"defab-erp/internal/role"

	"defab-erp/internal/branch"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"defab-erp/internal/user"
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

	roleStore := role.NewStore(database)
	roleHandler := role.NewHandler(roleStore)

	branchStore := branch.NewStore(database)
	branchHandler := branch.NewHandler(branchStore)

	warehouseStore := warehouse.NewStore(database)
	warehouseHandler := warehouse.NewHandler(warehouseStore)

	userStore := user.NewStore(database)
	userHandler := user.NewHandler(userStore)



	// 4. Fiber
	app := fiber.New()

	app.Use(logger.New())
	app.Use(recover.New())


	api := app.Group("/api")

	auth.RegisterRoutes(api, authHandler)

	// Quick test route for products
	api.Get("/products/test", func(c *fiber.Ctx) error {
		// Just to prove store works
		return c.SendString("Product Store Connected")
	})

	// ---------- PROTECTED ----------
	protected := api.Group("", middleware.JWTProtected(authStore))

	role.RegisterRoutes(
	protected.Group("",
		middleware.RequireRole(model.RoleSuperAdmin),
	),
	roleHandler,
	)


	branch.RegisterRoutes(
	protected.Group("",
		middleware.RequireRole(model.RoleSuperAdmin),
	),
	branchHandler,
	)


	warehouse.RegisterRoutes(
	protected.Group("",
		middleware.RequireRole(model.RoleSuperAdmin),
	),
	warehouseHandler,
	)

	user.RegisterRoutes(
	protected.Group("",
		middleware.RequireRole(model.RoleSuperAdmin),
	),
	userHandler,
	)





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

package main

import (
	"log"
	"os"

	"defab-erp/internal/auth"
	"defab-erp/internal/core/db"

	"defab-erp/internal/stocktransfer"

	"defab-erp/internal/warehouse"

	"defab-erp/internal/core/model"
	"defab-erp/internal/middleware"

	"defab-erp/internal/role"

	"defab-erp/internal/branch"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"defab-erp/internal/attribute"
	"defab-erp/internal/category"
	"defab-erp/internal/product"

	"defab-erp/internal/productdescription"

	"defab-erp/internal/user"
	"defab-erp/internal/variant"

	"defab-erp/internal/core/storage"

	"defab-erp/internal/coupon"
	"defab-erp/internal/goodsreceipt"
	"defab-erp/internal/purchase"
	"defab-erp/internal/purchaseinvoice"
	"defab-erp/internal/rawmaterial"
	"defab-erp/internal/stock"
	"defab-erp/internal/stockrequest"
	"defab-erp/internal/supplier"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("⚠ .env file not found, using system ENV")
	}
	// 1. DB
	database := db.Connect()
	defer database.Close()

	if err := storage.InitSpaces(); err != nil {
		log.Fatal("spaces init failed:", err)
	}

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

	categoryStore := category.NewStore(database)
	categoryHandler := category.NewHandler(categoryStore)

	productStore := product.NewStore(database)
	productHandler := product.NewHandler(productStore)

	pdStore := productdescription.NewStore(database)
	pdHandler := productdescription.NewHandler(pdStore)

	attributeStore := attribute.NewStore(database)
	attributeHandler := attribute.NewHandler(attributeStore)

	variantStore := variant.NewStore(database)
	variantHandler := variant.NewHandler(variantStore)

	supplierStore := supplier.NewStore(database)
	supplierHandler := supplier.NewHandler(supplierStore)

	purchaseStore := purchase.NewStore(database)
	purchaseHandler := purchase.NewHandler(purchaseStore)

	goodsStore := goodsreceipt.NewStore(database)
	goodsHandler := goodsreceipt.NewHandler(goodsStore)

	stockTransferStore := stocktransfer.NewStore(database)
	stockTransferHandler := stocktransfer.NewHandler(stockTransferStore)

	stockStore := stock.NewStore(database)
	stockHandler := stock.NewHandler(stockStore)

	stockRequestStore := stockrequest.NewStore(database)
	stockRequestHandler := stockrequest.NewHandler(stockRequestStore)

	couponStore := coupon.NewStore(database)
	couponHandler := coupon.NewHandler(couponStore)

	rawMaterialStore := rawmaterial.NewStore(database)
	rawMaterialHandler := rawmaterial.NewHandler(rawMaterialStore)

	purchaseInvoiceStore := purchaseinvoice.NewStore(database)
	purchaseInvoiceHandler := purchaseinvoice.NewHandler(purchaseInvoiceStore)

	// 4. Fiber
	app := fiber.New(fiber.Config{
		BodyLimit: 50 * 1024 * 1024, // 50 MB
	})

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173,https://defab-erp-frontend.vercel.app",
		AllowHeaders:     "*",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

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

	category.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		categoryHandler,
	)

	product.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		productHandler,
	)

	productdescription.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(model.RoleSuperAdmin),
		),
		pdHandler,
	)

	attribute.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		attributeHandler,
	)

	variant.RegisterRoutes(
		protected.Group("", middleware.RequireRole(
			model.RoleSuperAdmin,
			model.RoleInventoryManager,
		)),
		variantHandler,
	)

	supplier.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		supplierHandler,
	)

	purchase.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		purchaseHandler,
	)

	goodsreceipt.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		goodsHandler,
	)

	stocktransfer.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		stockTransferHandler,
	)

	stock.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
				model.RoleStoreManager,
			),
		),
		stockHandler,
	)

	stockrequest.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
				model.RoleStoreManager,
			),
		),
		stockRequestHandler,
	)

	coupon.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		couponHandler,
	)

	rawmaterial.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		rawMaterialHandler,
	)

	purchaseinvoice.RegisterRoutes(
		protected.Group("",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleInventoryManager,
			),
		),
		purchaseInvoiceHandler,
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

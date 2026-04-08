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

	"defab-erp/internal/accounting"
	"defab-erp/internal/billing"
	"defab-erp/internal/coupon"
	"defab-erp/internal/customer"
	"defab-erp/internal/dashboard"
	"defab-erp/internal/goodsreceipt"
	"defab-erp/internal/joborder"
	"defab-erp/internal/production"
	"defab-erp/internal/purchase"
	"defab-erp/internal/purchaseinvoice"
	"defab-erp/internal/rawmaterial"
	"defab-erp/internal/returns"
	"defab-erp/internal/salesinvoice"
	"defab-erp/internal/salesorder"
	"defab-erp/internal/salesperson"
	"defab-erp/internal/stock"
	"defab-erp/internal/stockrequest"
	"defab-erp/internal/supplier"

	ecomCart "defab-erp/internal/ecom/cart"
	ecomCustomer "defab-erp/internal/ecom/customer"
	ecomMw "defab-erp/internal/ecom/middleware"
	ecomOrder "defab-erp/internal/ecom/order"
	ecomProduct "defab-erp/internal/ecom/product"

	"defab-erp/internal/migration"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("⚠ .env file not found, using system ENV")
	}
	// 1. DB
	database := db.Connect()
	defer database.Close()

	// Redis (optional — nil means caching disabled)
	redisClient := db.ConnectRedis()
	if redisClient != nil {
		defer redisClient.Close()
	}

	log.Println("⏳ Initializing storage...")
	if err := storage.InitSpaces(); err != nil {
		log.Println("⚠ spaces init failed (continuing without cloud storage):", err)
	}
	log.Println("✅ Storage initialized")

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

	salespersonStore := salesperson.NewStore(database)
	salespersonHandler := salesperson.NewHandler(salespersonStore)

	customerStore := customer.NewStore(database)
	customerHandler := customer.NewHandler(customerStore)

	salesOrderStore := salesorder.NewStore(database)
	salesOrderHandler := salesorder.NewHandler(salesOrderStore)

	salesInvoiceStore := salesinvoice.NewStore(database)
	salesInvoiceHandler := salesinvoice.NewHandler(salesInvoiceStore)

	billingStore := billing.NewStore(database, redisClient)
	billingHandler := billing.NewHandler(billingStore)

	accountingStore := accounting.NewStore(database)
	accountingRecorder := accounting.NewRecorder(database, accountingStore)
	accountingHandler := accounting.NewHandler(accountingStore, accountingRecorder)

	returnStore := returns.NewStore(database)
	returnHandler := returns.NewHandler(returnStore, accountingRecorder)

	dashboardStore := dashboard.NewStore(database)
	dashboardHandler := dashboard.NewHandler(dashboardStore)

	jobOrderStore := joborder.NewStore(database)
	jobOrderHandler := joborder.NewHandler(jobOrderStore)

	productionStore := production.NewStore(database)
	productionHandler := production.NewHandler(productionStore)

	// ── Ecom stores & handlers ──
	ecomCustomerStore := ecomCustomer.NewStore(database)
	ecomCustomerHandler := ecomCustomer.NewHandler(ecomCustomerStore)

	ecomProductStore := ecomProduct.NewStore(database)
	ecomProductHandler := ecomProduct.NewHandler(ecomProductStore)

	ecomCartStore := ecomCart.NewStore(database)
	ecomCartHandler := ecomCart.NewHandler(ecomCartStore)

	ecomOrderStore := ecomOrder.NewStore(database)
	ecomOrderHandler := ecomOrder.NewHandler(ecomOrderStore)

	migrationStore := migration.NewStore(database)
	migrationHandler := migration.NewHandler(migrationStore)

	// Wire auto-recording into billing & purchase handlers
	billingHandler.SetRecorder(accountingRecorder)
	purchaseInvoiceHandler.SetRecorder(accountingRecorder)

	// Warm Redis cache with all variants (async — don't block server start)
	go func() {
		if err := billingStore.WarmCache(); err != nil {
			log.Println("⚠ Cache warm-up failed:", err)
		} else {
			log.Println("✅ Redis variant cache warmed")
		}
	}()

	// 4. Fiber
	app := fiber.New(fiber.Config{
		BodyLimit: 50 * 1024 * 1024, // 50 MB
	})

	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "*",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowCredentials: false,
	}))

	api := app.Group("/api")

	auth.RegisterRoutes(api, authHandler)

	// ═══════════════════════════════════════════
	// E-COMMERCE PUBLIC ROUTES (before protected group)
	// ═══════════════════════════════════════════
	ecom := api.Group("/ecom")
	ecomCustomer.RegisterPublicRoutes(ecom.Group("/auth"), ecomCustomerHandler)
	ecomProduct.RegisterRoutes(ecom.Group("/products"), ecomProductHandler)

	// E-COMMERCE PROTECTED ROUTES (before ERP protected group)
	ecomProtected := ecom.Group("", ecomMw.EcomJWTProtected(database))
	ecomCustomer.RegisterProtectedRoutes(ecomProtected, ecomCustomerHandler)
	ecomCart.RegisterRoutes(ecomProtected.Group("/cart"), ecomCartHandler)
	ecomOrder.RegisterCustomerRoutes(ecomProtected.Group("/orders"), ecomOrderHandler)

	// Quick test route for products
	api.Get("/products/test", func(c *fiber.Ctx) error {
		// Just to prove store works
		return c.SendString("Product Store Connected")
	})

	// ---------- PROTECTED ----------
	protected := api.Group("", middleware.JWTProtected(authStore))

	role.RegisterRoutes(
		protected.Group("/roles",
			middleware.RequireRole(model.RoleSuperAdmin),
		),
		roleHandler,
	)

	branch.RegisterRoutes(
		protected.Group("/branches",
			middleware.RequireRole(model.RoleSuperAdmin, model.RoleAccountsManager),
		),
		branchHandler,
	)

	warehouse.RegisterListRoute(
		protected.Group("/warehouses",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		warehouseHandler,
	)

	warehouse.RegisterRoutes(
		protected.Group("/warehouses",
			middleware.RequireRole(model.RoleSuperAdmin, model.RoleAccountsManager),
		),
		warehouseHandler,
	)

	user.RegisterRoutes(
		protected.Group("/users",
			middleware.RequireRole(model.RoleSuperAdmin),
		),
		userHandler,
	)

	category.RegisterRoutes(
		protected.Group("/categories",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		categoryHandler,
	)

	product.RegisterRoutes(
		protected.Group("/products",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		productHandler,
	)

	productdescription.RegisterRoutes(
		protected.Group("/product-descriptions",
			middleware.RequireRole(model.RoleSuperAdmin, model.RoleAccountsManager),
		),
		pdHandler,
	)

	attribute.RegisterRoutes(
		protected.Group("/attributes",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		attributeHandler,
	)

	variant.RegisterRoutes(
		protected.Group("/variants",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		variantHandler,
	)

	supplier.RegisterRoutes(
		protected.Group("/suppliers",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		supplierHandler,
	)

	salesperson.RegisterRoutes(
		protected.Group("/salespersons",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		salespersonHandler,
	)

	customer.RegisterRoutes(
		protected.Group("/customers",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleSalesPerson,
				model.RoleAccountsManager,
			),
		),
		customerHandler,
	)

	salesorder.RegisterRoutes(
		protected.Group("/sales-orders",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleSalesPerson,
				model.RoleAccountsManager,
			),
		),
		salesOrderHandler,
	)

	salesinvoice.RegisterRoutes(
		protected.Group("/sales-invoices",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleSalesPerson,
				model.RoleAccountsManager,
			),
		),
		salesInvoiceHandler,
	)

	billing.RegisterRoutes(
		protected.Group("/billing",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleSalesPerson,
				model.RoleAccountsManager,
			),
		),
		billingHandler,
	)

	returns.RegisterRoutes(
		protected.Group("/returns",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleSalesPerson,
				model.RoleAccountsManager,
			),
		),
		returnHandler,
	)

	purchase.RegisterRoutes(
		protected.Group("/purchase-orders",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		purchaseHandler,
	)

	goodsreceipt.RegisterRoutes(
		protected.Group("/goods-receipts",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		goodsHandler,
	)

	stocktransfer.RegisterRoutes(
		protected.Group("/stock-transfers",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		stockTransferHandler,
	)

	stock.RegisterRoutes(
		protected.Group("/stocks",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		stockHandler,
	)

	stockrequest.RegisterRoutes(
		protected.Group("/stock-requests",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		stockRequestHandler,
	)

	coupon.RegisterRoutes(
		protected.Group("/coupons",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		couponHandler,
	)

	rawmaterial.RegisterRoutes(
		protected.Group("/raw-material-stocks",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		rawMaterialHandler,
	)

	purchaseinvoice.RegisterInvoiceRoutes(
		protected.Group("/purchase-invoices",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		purchaseInvoiceHandler,
	)
	purchaseinvoice.RegisterPaymentRoutes(
		protected.Group("/supplier-payments",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		purchaseInvoiceHandler,
	)

	accounting.RegisterRoutes(
		protected.Group("/accounting",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleAccountsManager,
				model.RoleStoreManager,
			),
		),
		accountingHandler,
	)

	dashboard.RegisterRoutes(
		protected.Group("/dashboard",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleAccountsManager,
				model.RoleStoreManager,
			),
		),
		dashboardHandler,
	)

	joborder.RegisterRoutes(
		protected.Group("/job-orders",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		jobOrderHandler,
	)

	production.RegisterRoutes(
		protected.Group("/production-orders",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
				model.RoleAccountsManager,
			),
		),
		productionHandler,
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

	// ═══════════════════════════════════════════
	// E-COMMERCE ADMIN ROUTES (ERP staff managing ecom orders)
	// ═══════════════════════════════════════════

	migration.RegisterRoutes(
		protected.Group("/migration",
			middleware.RequireRole(model.RoleSuperAdmin),
		),
		migrationHandler,
	)

	// Admin: ERP staff managing ecom orders
	ecomOrder.RegisterAdminRoutes(
		protected.Group("/ecom-orders",
			middleware.RequireRole(
				model.RoleSuperAdmin,
				model.RoleStoreManager,
			),
		),
		ecomOrderHandler,
	)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))
}

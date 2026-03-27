package accounting

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(r fiber.Router, h *Handler) {
	// ── Chart of Accounts ──
	r.Get("/account-groups", h.ListAccountGroups)
	r.Post("/account-groups", h.CreateAccountGroup)

	r.Get("/accounts", h.ListLedgerAccounts)
	r.Post("/accounts", h.CreateLedgerAccount)
	r.Get("/accounts/:id", h.GetLedgerAccount)

	// ── Financial Years ──
	r.Get("/financial-years", h.ListFinancialYears)
	r.Post("/financial-years", h.CreateFinancialYear)

	// ── Vouchers (manual journal entries) ──
	r.Get("/vouchers", h.ListVouchers)
	r.Post("/vouchers", h.CreateVoucher)
	r.Get("/vouchers/:id", h.GetVoucher)
	r.Delete("/vouchers/:id", h.CancelVoucher)

	// ── Auto-record from existing transactions ──
	r.Post("/record/sales-invoice/:id", h.RecordSalesInvoice)
	r.Post("/record/purchase-invoice/:id", h.RecordPurchaseInvoice)
	r.Post("/record/sales-payment/:id", h.RecordSalesPayment)
	r.Post("/record/supplier-payment/:id", h.RecordSupplierPayment)
	r.Post("/backfill", h.Backfill)

	// ── Reports ──
	r.Get("/reports/trial-balance", h.TrialBalance)
	r.Get("/reports/ledger/:accountId", h.Ledger)
	r.Get("/reports/profit-loss", h.ProfitAndLoss)
	r.Get("/reports/balance-sheet", h.BalanceSheet)
	r.Get("/reports/day-book", h.DayBook)
}

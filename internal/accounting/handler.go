package accounting

import (
	"database/sql"
	"log"
	"strconv"

	"defab-erp/internal/core/httperr"
	"defab-erp/internal/core/model"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store    *Store
	recorder *Recorder
}

func NewHandler(s *Store, r *Recorder) *Handler {
	return &Handler{store: s, recorder: r}
}

// ════════════════════════════════════════════
// Account Groups
// ════════════════════════════════════════════

func (h *Handler) CreateAccountGroup(c *fiber.Ctx) error {
	var in CreateAccountGroupInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Name == "" || in.Nature == "" {
		return httperr.BadRequest(c, "name and nature are required")
	}
	id, err := h.store.CreateAccountGroup(in)
	if err != nil {
		log.Println("create account group error:", err)
		return httperr.Internal(c)
	}
	return c.Status(201).JSON(fiber.Map{"id": id, "message": "Account group created"})
}

func (h *Handler) ListAccountGroups(c *fiber.Ctx) error {
	groups, err := h.store.ListAccountGroups()
	if err != nil {
		log.Println("list account groups error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"account_groups": groups})
}

// ════════════════════════════════════════════
// Ledger Accounts
// ════════════════════════════════════════════

func (h *Handler) CreateLedgerAccount(c *fiber.Ctx) error {
	var in CreateLedgerAccountInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Code == "" || in.Name == "" || in.AccountGroupID == "" || in.Nature == "" {
		return httperr.BadRequest(c, "code, name, account_group_id, and nature are required")
	}
	if in.Nature != "DEBIT" && in.Nature != "CREDIT" {
		return httperr.BadRequest(c, "nature must be DEBIT or CREDIT")
	}
	id, err := h.store.CreateLedgerAccount(in)
	if err != nil {
		log.Println("create ledger account error:", err)
		return httperr.Internal(c)
	}
	return c.Status(201).JSON(fiber.Map{"id": id, "message": "Ledger account created"})
}

func (h *Handler) ListLedgerAccounts(c *fiber.Ctx) error {
	accounts, err := h.store.ListLedgerAccounts()
	if err != nil {
		log.Println("list ledger accounts error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"accounts": accounts})
}

func (h *Handler) GetLedgerAccount(c *fiber.Ctx) error {
	id := c.Params("id")
	account, err := h.store.GetLedgerAccount(id)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Ledger account not found")
	}
	if err != nil {
		log.Println("get ledger account error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(account)
}

// ════════════════════════════════════════════
// Financial Years
// ════════════════════════════════════════════

func (h *Handler) CreateFinancialYear(c *fiber.Ctx) error {
	var in CreateFinancialYearInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.Name == "" || in.StartDate == "" || in.EndDate == "" {
		return httperr.BadRequest(c, "name, start_date, and end_date are required")
	}
	id, err := h.store.CreateFinancialYear(in)
	if err != nil {
		log.Println("create financial year error:", err)
		return httperr.Internal(c)
	}
	return c.Status(201).JSON(fiber.Map{"id": id, "message": "Financial year created"})
}

func (h *Handler) ListFinancialYears(c *fiber.Ctx) error {
	years, err := h.store.ListFinancialYears()
	if err != nil {
		log.Println("list financial years error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"financial_years": years})
}

// ════════════════════════════════════════════
// Vouchers
// ════════════════════════════════════════════

func (h *Handler) CreateVoucher(c *fiber.Ctx) error {
	var in CreateVoucherInput
	if err := c.BodyParser(&in); err != nil {
		return httperr.BadRequest(c, "Invalid JSON body")
	}
	if in.VoucherType == "" || in.VoucherDate == "" {
		return httperr.BadRequest(c, "voucher_type and voucher_date are required")
	}
	if len(in.Lines) < 2 {
		return httperr.BadRequest(c, "at least 2 lines (debit + credit) are required")
	}

	user := c.Locals("user").(*model.User)

	var lines []VoucherLine
	for _, l := range in.Lines {
		if l.LedgerAccountID == "" {
			return httperr.BadRequest(c, "ledger_account_id is required for all lines")
		}
		lines = append(lines, VoucherLine{
			LedgerAccountID: l.LedgerAccountID,
			Debit:           l.Debit,
			Credit:          l.Credit,
			Narration:       l.Narration,
		})
	}

	err := h.store.CreateVoucher(Voucher{
		VoucherType: in.VoucherType,
		VoucherDate: in.VoucherDate,
		Narration:   in.Narration,
		BranchID:    in.BranchID,
		CreatedBy:   user.ID.String(),
		Lines:       lines,
	})
	if err != nil {
		log.Println("create voucher error:", err)
		if err.Error()[:18] == "voucher unbalanced" {
			return httperr.BadRequest(c, err.Error())
		}
		return httperr.Internal(c)
	}
	return c.Status(201).JSON(fiber.Map{"message": "Voucher created"})
}

func (h *Handler) GetVoucher(c *fiber.Ctx) error {
	id := c.Params("id")
	v, err := h.store.GetVoucher(id)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Voucher not found")
	}
	if err != nil {
		log.Println("get voucher error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(v)
}

func (h *Handler) ListVouchers(c *fiber.Ctx) error {
	voucherType := c.Query("type")
	from := c.Query("from")
	to := c.Query("to")
	branchID := c.Query("branch_id")
	search := c.Query("search")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	vouchers, total, err := h.store.ListVouchers(voucherType, from, to, branchID, search, limit, offset)
	if err != nil {
		log.Println("list vouchers error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{
		"vouchers": vouchers,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *Handler) CancelVoucher(c *fiber.Ctx) error {
	id := c.Params("id")
	err := h.store.CancelVoucher(id)
	if err == sql.ErrNoRows {
		return httperr.NotFound(c, "Voucher not found")
	}
	if err != nil {
		log.Println("cancel voucher error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Voucher cancelled"})
}

// ════════════════════════════════════════════
// Recording (Business Event → Voucher)
// ════════════════════════════════════════════

func (h *Handler) RecordSalesInvoice(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.recorder.RecordSalesInvoice(id, user.ID.String()); err != nil {
		log.Println("record sales invoice error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Sales invoice recorded in accounting"})
}

func (h *Handler) RecordPurchaseInvoice(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.recorder.RecordPurchaseInvoice(id, user.ID.String()); err != nil {
		log.Println("record purchase invoice error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Purchase invoice recorded in accounting"})
}

func (h *Handler) RecordSalesPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.recorder.RecordSalesPayment(id, user.ID.String()); err != nil {
		log.Println("record sales payment error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Sales payment recorded in accounting"})
}

func (h *Handler) RecordSupplierPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	user := c.Locals("user").(*model.User)
	if err := h.recorder.RecordSupplierPayment(id, user.ID.String()); err != nil {
		log.Println("record supplier payment error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"message": "Supplier payment recorded in accounting"})
}

func (h *Handler) Backfill(c *fiber.Ctx) error {
	user := c.Locals("user").(*model.User)
	result, err := h.recorder.BackfillAll(user.ID.String())
	if err != nil {
		log.Println("backfill error:", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error(), "partial_result": result})
	}
	return c.JSON(fiber.Map{"message": "Backfill completed", "recorded": result})
}

// ════════════════════════════════════════════
// Reports
// ════════════════════════════════════════════

func (h *Handler) TrialBalance(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")

	if fyID := c.Query("financial_year_id"); fyID != "" {
		start, end, err := h.store.GetFinancialYearDates(fyID)
		if err != nil {
			return httperr.BadRequest(c, "Invalid financial_year_id")
		}
		from, to = start, end
	}

	// Backward compat: as_of sets the to-date
	if asOf := c.Query("as_of"); asOf != "" && to == "" {
		to = asOf
	}

	rows, err := h.store.TrialBalance(from, to)
	if err != nil {
		log.Println("trial balance error:", err)
		return httperr.Internal(c)
	}

	var totalDebit, totalCredit float64
	for _, r := range rows {
		totalDebit += r.TotalDebit
		totalCredit += r.TotalCredit
	}

	return c.JSON(fiber.Map{
		"trial_balance": rows,
		"total_debit":   totalDebit,
		"total_credit":  totalCredit,
		"from":          from,
		"to":            to,
	})
}

func (h *Handler) Ledger(c *fiber.Ctx) error {
	accountID := c.Params("accountId")
	from := c.Query("from")
	to := c.Query("to")

	if fyID := c.Query("financial_year_id"); fyID != "" {
		start, end, err := h.store.GetFinancialYearDates(fyID)
		if err != nil {
			return httperr.BadRequest(c, "Invalid financial_year_id")
		}
		from, to = start, end
	}

	entries, err := h.store.Ledger(accountID, from, to)
	if err != nil {
		log.Println("ledger error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{
		"account_id": accountID,
		"entries":    entries,
		"from":       from,
		"to":         to,
	})
}

func (h *Handler) ProfitAndLoss(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")

	if fyID := c.Query("financial_year_id"); fyID != "" {
		start, end, err := h.store.GetFinancialYearDates(fyID)
		if err != nil {
			return httperr.BadRequest(c, "Invalid financial_year_id")
		}
		from, to = start, end
	}

	result, err := h.store.ProfitAndLoss(from, to)
	if err != nil {
		log.Println("P&L error:", err)
		return httperr.Internal(c)
	}
	result["from"] = from
	result["to"] = to
	return c.JSON(result)
}

func (h *Handler) BalanceSheet(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")

	if fyID := c.Query("financial_year_id"); fyID != "" {
		start, end, err := h.store.GetFinancialYearDates(fyID)
		if err != nil {
			return httperr.BadRequest(c, "Invalid financial_year_id")
		}
		from, to = start, end
	}

	// Backward compat: as_of sets the to-date
	if asOf := c.Query("as_of"); asOf != "" && to == "" {
		to = asOf
	}

	result, err := h.store.BalanceSheet(from, to)
	if err != nil {
		log.Println("balance sheet error:", err)
		return httperr.Internal(c)
	}
	result["from"] = from
	result["to"] = to
	return c.JSON(result)
}

func (h *Handler) DayBook(c *fiber.Ctx) error {
	date := c.Query("date")

	// If financial_year_id is passed without date, return error
	if fyID := c.Query("financial_year_id"); fyID != "" && date == "" {
		return httperr.BadRequest(c, "date query param is required for day-book (YYYY-MM-DD)")
	}

	if date == "" {
		return httperr.BadRequest(c, "date query param is required (YYYY-MM-DD)")
	}
	entries, err := h.store.DayBook(date)
	if err != nil {
		log.Println("day book error:", err)
		return httperr.Internal(c)
	}
	return c.JSON(fiber.Map{"date": date, "entries": entries})
}

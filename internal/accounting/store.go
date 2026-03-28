package accounting

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ════════════════════════════════════════════
// Account Groups
// ════════════════════════════════════════════

func (s *Store) CreateAccountGroup(in CreateAccountGroupInput) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO account_groups (name, parent_id, nature)
		VALUES ($1, $2, $3) RETURNING id
	`, in.Name, in.ParentID, in.Nature).Scan(&id)
	return id, err
}

func (s *Store) ListAccountGroups() ([]AccountGroup, error) {
	rows, err := s.db.Query(`
		SELECT g.id, g.name, g.parent_id, p.name, g.nature, g.created_at
		FROM account_groups g
		LEFT JOIN account_groups p ON p.id = g.parent_id
		ORDER BY g.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []AccountGroup
	for rows.Next() {
		var g AccountGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.ParentID, &g.ParentName, &g.Nature, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// ════════════════════════════════════════════
// Ledger Accounts
// ════════════════════════════════════════════

func (s *Store) CreateLedgerAccount(in CreateLedgerAccountInput) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO ledger_accounts (code, name, account_group_id, nature, description)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, in.Code, in.Name, in.AccountGroupID, in.Nature, in.Description).Scan(&id)
	return id, err
}

func (s *Store) ListLedgerAccounts() ([]LedgerAccount, error) {
	rows, err := s.db.Query(`
		SELECT la.id, la.code, la.name, la.account_group_id, ag.name,
		       la.nature, la.is_system, la.is_active, COALESCE(la.description,''), la.created_at
		FROM ledger_accounts la
		JOIN account_groups ag ON ag.id = la.account_group_id
		ORDER BY la.code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []LedgerAccount
	for rows.Next() {
		var a LedgerAccount
		if err := rows.Scan(&a.ID, &a.Code, &a.Name, &a.AccountGroupID, &a.GroupName,
			&a.Nature, &a.IsSystem, &a.IsActive, &a.Description, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Store) GetLedgerAccount(id string) (*LedgerAccount, error) {
	var a LedgerAccount
	err := s.db.QueryRow(`
		SELECT la.id, la.code, la.name, la.account_group_id, ag.name,
		       la.nature, la.is_system, la.is_active, COALESCE(la.description,''), la.created_at
		FROM ledger_accounts la
		JOIN account_groups ag ON ag.id = la.account_group_id
		WHERE la.id = $1
	`, id).Scan(&a.ID, &a.Code, &a.Name, &a.AccountGroupID, &a.GroupName,
		&a.Nature, &a.IsSystem, &a.IsActive, &a.Description, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ════════════════════════════════════════════
// Financial Years
// ════════════════════════════════════════════

func (s *Store) CreateFinancialYear(in CreateFinancialYearInput) (string, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO financial_years (name, start_date, end_date)
		VALUES ($1, $2, $3) RETURNING id
	`, in.Name, in.StartDate, in.EndDate).Scan(&id)
	return id, err
}

func (s *Store) ListFinancialYears() ([]FinancialYear, error) {
	rows, err := s.db.Query(`
		SELECT id, name, start_date, end_date, is_active, is_closed, created_at
		FROM financial_years ORDER BY start_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var years []FinancialYear
	for rows.Next() {
		var fy FinancialYear
		if err := rows.Scan(&fy.ID, &fy.Name, &fy.StartDate, &fy.EndDate,
			&fy.IsActive, &fy.IsClosed, &fy.CreatedAt); err != nil {
			return nil, err
		}
		years = append(years, fy)
	}
	return years, nil
}

func (s *Store) GetActiveFinancialYear(date time.Time) (string, error) {
	var id string
	err := s.db.QueryRow(`
		SELECT id FROM financial_years
		WHERE is_active = TRUE AND is_closed = FALSE
		  AND start_date <= $1 AND end_date >= $1
		LIMIT 1
	`, date).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// GetFinancialYearDates returns start_date and end_date for a given financial year ID.
func (s *Store) GetFinancialYearDates(fyID string) (string, string, error) {
	var start, end string
	err := s.db.QueryRow(`SELECT start_date::text, end_date::text FROM financial_years WHERE id = $1`, fyID).Scan(&start, &end)
	return start, end, err
}

// ════════════════════════════════════════════
// Vouchers
// ════════════════════════════════════════════

func (s *Store) nextVoucherNumber(voucherType string) (string, error) {
	prefix := "JV"
	switch voucherType {
	case VoucherTypeSales:
		prefix = "SV"
	case VoucherTypePurchase:
		prefix = "PV"
	case VoucherTypeReceipt:
		prefix = "RV"
	case VoucherTypePayment:
		prefix = "PMV"
	case VoucherTypeContra:
		prefix = "CV"
	}

	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM vouchers WHERE voucher_type = $1`, voucherType).Scan(&count)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%05d", prefix, count+1), nil
}

// CreateVoucher inserts a voucher with its lines inside a transaction.
// It validates that total debits == total credits.
func (s *Store) CreateVoucher(v Voucher) error {
	var totalDebit, totalCredit float64
	for _, l := range v.Lines {
		totalDebit += l.Debit
		totalCredit += l.Credit
	}
	if math.Abs(totalDebit-totalCredit) > 0.01 {
		return fmt.Errorf("voucher unbalanced: debit=%.2f credit=%.2f", totalDebit, totalCredit)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	voucherNum := v.VoucherNumber
	if voucherNum == "" {
		voucherNum, err = s.nextVoucherNumber(v.VoucherType)
		if err != nil {
			return fmt.Errorf("generate voucher number: %w", err)
		}
	}

	fyID, _ := s.GetActiveFinancialYear(time.Now())

	var fyParam interface{}
	if fyID != "" {
		fyParam = fyID
	}
	var branchParam interface{}
	if v.BranchID != "" {
		branchParam = v.BranchID
	}
	var refTypeParam, refIDParam interface{}
	if v.RefType != "" {
		refTypeParam = v.RefType
	}
	if v.RefID != "" {
		refIDParam = v.RefID
	}
	var createdByParam interface{}
	if v.CreatedBy != "" {
		createdByParam = v.CreatedBy
	}

	var voucherDate interface{}
	if v.VoucherDate != "" {
		voucherDate = v.VoucherDate
	} else {
		voucherDate = time.Now().Format("2006-01-02")
	}

	var voucherID string
	err = tx.QueryRow(`
		INSERT INTO vouchers
			(voucher_number, voucher_type, voucher_date, narration,
			 ref_type, ref_id, financial_year_id, branch_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, voucherNum, v.VoucherType, voucherDate, v.Narration,
		refTypeParam, refIDParam, fyParam, branchParam, createdByParam).Scan(&voucherID)
	if err != nil {
		return fmt.Errorf("insert voucher: %w", err)
	}

	for _, l := range v.Lines {
		if l.Debit == 0 && l.Credit == 0 {
			continue
		}
		_, err = tx.Exec(`
			INSERT INTO voucher_lines (voucher_id, ledger_account_id, debit, credit, narration)
			VALUES ($1, $2, $3, $4, $5)
		`, voucherID, l.LedgerAccountID, l.Debit, l.Credit, l.Narration)
		if err != nil {
			return fmt.Errorf("insert voucher line: %w", err)
		}
	}

	return tx.Commit()
}

// VoucherExistsForRef checks if a voucher has already been created for
// a given ref_type + ref_id (idempotency).
func (s *Store) VoucherExistsForRef(refType, refID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM vouchers WHERE ref_type=$1 AND ref_id=$2 AND is_cancelled=FALSE)
	`, refType, refID).Scan(&exists)
	return exists, err
}

// GetVoucher returns a single voucher with all its lines.
func (s *Store) GetVoucher(id string) (*Voucher, error) {
	var v Voucher
	var refType, refID, fyID, branchID, createdBy sql.NullString
	err := s.db.QueryRow(`
		SELECT id, voucher_number, voucher_type, voucher_date, COALESCE(narration,''),
		       ref_type, ref_id, financial_year_id, branch_id, is_cancelled, created_by, created_at
		FROM vouchers WHERE id = $1
	`, id).Scan(&v.ID, &v.VoucherNumber, &v.VoucherType, &v.VoucherDate, &v.Narration,
		&refType, &refID, &fyID, &branchID, &v.IsCancelled, &createdBy, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	if refType.Valid {
		v.RefType = refType.String
	}
	if refID.Valid {
		v.RefID = refID.String
	}
	if fyID.Valid {
		v.FinancialYearID = fyID.String
	}
	if branchID.Valid {
		v.BranchID = branchID.String
	}
	if createdBy.Valid {
		v.CreatedBy = createdBy.String
	}

	rows, err := s.db.Query(`
		SELECT vl.id, vl.ledger_account_id, la.name, la.code,
		       vl.debit, vl.credit, COALESCE(vl.narration,'')
		FROM voucher_lines vl
		JOIN ledger_accounts la ON la.id = vl.ledger_account_id
		WHERE vl.voucher_id = $1
		ORDER BY vl.created_at
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var l VoucherLine
		if err := rows.Scan(&l.ID, &l.LedgerAccountID, &l.AccountName, &l.AccountCode,
			&l.Debit, &l.Credit, &l.Narration); err != nil {
			return nil, err
		}
		v.TotalDebit += l.Debit
		v.TotalCredit += l.Credit
		v.Lines = append(v.Lines, l)
	}
	return &v, nil
}

// ListVouchers returns vouchers filtered by type, date range, branch.
func (s *Store) ListVouchers(voucherType, from, to, branchID, search string, limit, offset int) ([]Voucher, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	idx := 1

	if voucherType != "" {
		where += fmt.Sprintf(" AND v.voucher_type = $%d", idx)
		args = append(args, voucherType)
		idx++
	}
	if from != "" {
		where += fmt.Sprintf(" AND v.voucher_date >= $%d", idx)
		args = append(args, from)
		idx++
	}
	if to != "" {
		where += fmt.Sprintf(" AND v.voucher_date <= $%d", idx)
		args = append(args, to)
		idx++
	}
	if branchID != "" {
		where += fmt.Sprintf(" AND v.branch_id = $%d", idx)
		args = append(args, branchID)
		idx++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (v.voucher_number ILIKE $%d OR v.narration ILIKE $%d)", idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	// Count total
	var total int
	countQ := "SELECT COUNT(*) FROM vouchers v " + where
	err := s.db.QueryRow(countQ, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT v.id, v.voucher_number, v.voucher_type, v.voucher_date,
		       COALESCE(v.narration,''), COALESCE(v.ref_type,''), COALESCE(v.ref_id::text,''),
		       COALESCE(v.financial_year_id::text,''), COALESCE(fy.name,''),
		       COALESCE(v.branch_id::text,''), COALESCE(b.name,''),
		       COALESCE(v.created_by::text,''),
		       v.is_cancelled, v.created_at,
		       COALESCE(SUM(vl.debit),0), COALESCE(SUM(vl.credit),0)
		FROM vouchers v
		LEFT JOIN voucher_lines vl ON vl.voucher_id = v.id
		LEFT JOIN financial_years fy ON fy.id = v.financial_year_id
		LEFT JOIN branches b ON b.id = v.branch_id
		%s
		GROUP BY v.id, fy.name, b.name
		ORDER BY v.voucher_date DESC, v.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var vouchers []Voucher
	voucherIDs := []string{}
	for rows.Next() {
		var v Voucher
		if err := rows.Scan(&v.ID, &v.VoucherNumber, &v.VoucherType, &v.VoucherDate,
			&v.Narration, &v.RefType, &v.RefID,
			&v.FinancialYearID, &v.FinancialYearName,
			&v.BranchID, &v.BranchName, &v.CreatedBy,
			&v.IsCancelled, &v.CreatedAt, &v.TotalDebit, &v.TotalCredit); err != nil {
			return nil, 0, err
		}
		vouchers = append(vouchers, v)
		voucherIDs = append(voucherIDs, v.ID)
	}

	// Batch-fetch lines for all vouchers
	if len(voucherIDs) > 0 {
		placeholders := ""
		lineArgs := []interface{}{}
		for i, id := range voucherIDs {
			if i > 0 {
				placeholders += ","
			}
			placeholders += fmt.Sprintf("$%d", i+1)
			lineArgs = append(lineArgs, id)
		}
		lineRows, err := s.db.Query(fmt.Sprintf(`
			SELECT vl.id, vl.voucher_id, vl.ledger_account_id,
			       COALESCE(la.name,''), COALESCE(la.code,''),
			       vl.debit, vl.credit, COALESCE(vl.narration,'')
			FROM voucher_lines vl
			LEFT JOIN ledger_accounts la ON la.id = vl.ledger_account_id
			WHERE vl.voucher_id IN (%s)
			ORDER BY vl.debit DESC
		`, placeholders), lineArgs...)
		if err != nil {
			return nil, 0, err
		}
		defer lineRows.Close()

		lineMap := map[string][]VoucherLine{}
		for lineRows.Next() {
			var l VoucherLine
			if err := lineRows.Scan(&l.ID, &l.VoucherID, &l.LedgerAccountID,
				&l.AccountName, &l.AccountCode,
				&l.Debit, &l.Credit, &l.Narration); err != nil {
				return nil, 0, err
			}
			lineMap[l.VoucherID] = append(lineMap[l.VoucherID], l)
		}
		for i := range vouchers {
			vouchers[i].Lines = lineMap[vouchers[i].ID]
		}
	}

	return vouchers, total, nil
}

// CancelVoucher marks a voucher as cancelled (soft delete).
func (s *Store) CancelVoucher(id string) error {
	res, err := s.db.Exec(`UPDATE vouchers SET is_cancelled = TRUE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

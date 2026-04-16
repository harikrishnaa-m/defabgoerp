package attendance

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"defab-erp/internal/auth"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ──────────────────────────────────────────
// Punch In  (supports multiple sessions & branches per day)
// ──────────────────────────────────────────

func (s *Store) PunchIn(userID, branchID, notes string) (map[string]interface{}, error) {
	today := time.Now().Format("2006-01-02")

	// Block if user already has an open session (punched in, not yet out)
	var openID string
	err := s.db.QueryRow(
		`SELECT id FROM attendance WHERE user_id = $1 AND date = $2 AND punch_out IS NULL LIMIT 1`,
		userID, today,
	).Scan(&openID)
	if err == nil {
		return nil, fmt.Errorf("already punched in today")
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	var brParam interface{}
	if branchID != "" {
		brParam = branchID
	}

	// Next session_seq for this user+branch+date
	var maxSeq int
	s.db.QueryRow(
		`SELECT COALESCE(MAX(session_seq), 0) FROM attendance
		 WHERE user_id = $1 AND date = $2 AND branch_id IS NOT DISTINCT FROM $3::uuid`,
		userID, today, brParam,
	).Scan(&maxSeq)
	nextSeq := maxSeq + 1

	var id string
	var punchIn time.Time
	err = s.db.QueryRow(`
		INSERT INTO attendance (user_id, branch_id, date, punch_in, notes, session_seq, source)
		VALUES ($1, $2, $3, NOW(), $4, $5, 'self')
		RETURNING id, punch_in
	`, userID, brParam, today, notes, nextSeq).Scan(&id, &punchIn)
	if err != nil {
		return nil, fmt.Errorf("punch in: %w", err)
	}

	return map[string]interface{}{
		"id":          id,
		"user_id":     userID,
		"branch_id":   branchID,
		"date":        today,
		"punch_in":    punchIn,
		"punch_out":   nil,
		"session_seq": nextSeq,
		"notes":       notes,
	}, nil
}

// ──────────────────────────────────────────
// Punch Out  (closes the latest open session)
// ──────────────────────────────────────────

func (s *Store) PunchOut(userID, notes string) (map[string]interface{}, error) {
	today := time.Now().Format("2006-01-02")

	// Find the latest open session for today (any branch)
	var id string
	var punchIn time.Time
	err := s.db.QueryRow(`
		SELECT id, punch_in FROM attendance
		WHERE user_id = $1 AND date = $2 AND punch_out IS NULL
		ORDER BY punch_in DESC LIMIT 1
	`, userID, today).Scan(&id, &punchIn)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no punch-in found for today")
	}
	if err != nil {
		return nil, err
	}

	now := time.Now()
	hours := math.Round(now.Sub(punchIn).Hours()*100) / 100

	if notes != "" {
		_, err = s.db.Exec(
			`UPDATE attendance SET punch_out = NOW(), total_hours = $1, notes = $2 WHERE id = $3`,
			hours, notes, id,
		)
	} else {
		_, err = s.db.Exec(
			`UPDATE attendance SET punch_out = NOW(), total_hours = $1 WHERE id = $2`,
			hours, id,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("punch out: %w", err)
	}

	return map[string]interface{}{
		"id":          id,
		"user_id":     userID,
		"date":        today,
		"punch_in":    punchIn,
		"punch_out":   now,
		"total_hours": hours,
		"notes":       notes,
	}, nil
}

// ──────────────────────────────────────────
// List — today's attendance for all users (aggregated across sessions)
// ──────────────────────────────────────────

func (s *Store) List(userID *string, branchID *string, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	today := time.Now().Format("2006-01-02")

	where := "WHERE 1=1"
	args := []interface{}{}
	n := 0

	if userID != nil && *userID != "" {
		n++
		where += fmt.Sprintf(" AND u.id = $%d", n)
		args = append(args, *userID)
	}
	if branchID != nil && *branchID != "" {
		n++
		where += fmt.Sprintf(" AND u.branch_id = $%d", n)
		args = append(args, *branchID)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND u.name ILIKE $%d", n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM users u %s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	datePH := n
	args = append(args, today)

	n++
	limitP := n
	n++
	offsetP := n
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT u.id, u.name, u.branch_id, COALESCE(b.name,'') AS branch_name,
		       COALESCE(r.name,'') AS role_name,
		       COALESCE(att.sessions, 0),
		       att.first_in, att.last_out,
		       COALESCE(att.total_hours, 0),
		       COALESCE(att.has_open, false)
		FROM users u
		LEFT JOIN branches b ON b.id = u.branch_id
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN LATERAL (
		    SELECT COUNT(*)                     AS sessions,
		           MIN(a.punch_in)              AS first_in,
		           MAX(a.punch_out)             AS last_out,
		           SUM(COALESCE(a.total_hours,0)) AS total_hours,
		           BOOL_OR(a.punch_out IS NULL) AS has_open
		    FROM attendance a
		    WHERE a.user_id = u.id AND a.date = $%d
		) att ON true
		%s
		ORDER BY u.name
		LIMIT $%d OFFSET $%d
	`, datePH, where, limitP, offsetP)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var (
			uid, uname   string
			brID, brName sql.NullString
			roleName     string
			sessions     int
			firstIn      sql.NullTime
			lastOut      sql.NullTime
			totalHours   float64
			hasOpen      bool
		)
		if err := rows.Scan(&uid, &uname, &brID, &brName, &roleName,
			&sessions, &firstIn, &lastOut, &totalHours, &hasOpen); err != nil {
			return nil, 0, err
		}

		status := "absent"
		if sessions > 0 {
			status = "present"
		}
		if hasOpen {
			status = "clocked_in"
		}

		row := map[string]interface{}{
			"user_id":     uid,
			"user_name":   uname,
			"branch_id":   brID.String,
			"branch_name": brName.String,
			"role":        roleName,
			"date":        today,
			"punch_in":    nil,
			"punch_out":   nil,
			"total_hours": totalHours,
			"sessions":    sessions,
			"status":      status,
		}
		if firstIn.Valid {
			row["punch_in"] = firstIn.Time
		}
		if lastOut.Valid {
			row["punch_out"] = lastOut.Time
		}
		list = append(list, row)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID — date-range attendance for a user (multi-session aware)
// ──────────────────────────────────────────

func (s *Store) GetByID(targetUserID string, dateFrom, dateTo string) (map[string]interface{}, error) {
	// Fetch user info
	var uname string
	var brID, brName sql.NullString
	var roleName string
	err := s.db.QueryRow(`
		SELECT u.name, u.branch_id, COALESCE(b.name,''), COALESCE(r.name,'')
		FROM users u
		LEFT JOIN branches b ON b.id = u.branch_id
		LEFT JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1
	`, targetUserID).Scan(&uname, &brID, &brName, &roleName)
	if err != nil {
		return nil, err
	}

	// Default date range: current month
	if dateFrom == "" {
		now := time.Now()
		dateFrom = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}
	if dateTo == "" {
		dateTo = time.Now().Format("2006-01-02")
	}

	// Fetch all attendance records (sessions) in range
	rows, err := s.db.Query(`
		SELECT a.date::text, a.punch_in, a.punch_out, a.total_hours, a.notes,
		       a.session_seq, COALESCE(br.name, '') AS punch_branch
		FROM attendance a
		LEFT JOIN branches br ON br.id = a.branch_id
		WHERE a.user_id = $1 AND a.date >= $2 AND a.date <= $3
		ORDER BY a.date, a.session_seq
	`, targetUserID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group sessions by date
	type session struct {
		PunchIn    interface{} `json:"punch_in"`
		PunchOut   interface{} `json:"punch_out"`
		TotalHours float64     `json:"total_hours"`
		Notes      string      `json:"notes"`
		SessionSeq int         `json:"session_seq"`
		BranchName string      `json:"branch_name"`
	}
	dateSessionsMap := map[string][]session{}
	dateHoursMap := map[string]float64{}

	for rows.Next() {
		var date, notes, branchName string
		var punchIn time.Time
		var punchOut sql.NullTime
		var totalHours float64
		var sessionSeq int
		if err := rows.Scan(&date, &punchIn, &punchOut, &totalHours, &notes, &sessionSeq, &branchName); err != nil {
			continue
		}
		sess := session{
			PunchIn:    punchIn,
			TotalHours: totalHours,
			Notes:      notes,
			SessionSeq: sessionSeq,
			BranchName: branchName,
		}
		if punchOut.Valid {
			sess.PunchOut = punchOut.Time
		}
		dateSessionsMap[date] = append(dateSessionsMap[date], sess)
		dateHoursMap[date] += totalHours
	}

	// Generate every date in range
	start, _ := time.Parse("2006-01-02", dateFrom)
	end, _ := time.Parse("2006-01-02", dateTo)

	var days []map[string]interface{}
	var totalHoursSum float64
	var daysPresent, daysAbsent int

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		day := map[string]interface{}{
			"date":        ds,
			"status":      "absent",
			"sessions":    []session{},
			"punch_in":    nil,
			"punch_out":   nil,
			"total_hours": 0.0,
		}
		if sessions, ok := dateSessionsMap[ds]; ok && len(sessions) > 0 {
			day["status"] = "present"
			day["sessions"] = sessions
			day["total_hours"] = math.Round(dateHoursMap[ds]*100) / 100
			// Backward-compat: first punch_in and last punch_out
			day["punch_in"] = sessions[0].PunchIn
			if last := sessions[len(sessions)-1]; last.PunchOut != nil {
				day["punch_out"] = last.PunchOut
			}
			totalHoursSum += dateHoursMap[ds]
			daysPresent++
		} else {
			daysAbsent++
		}
		days = append(days, day)
	}

	return map[string]interface{}{
		"user_id":      targetUserID,
		"user_name":    uname,
		"branch_id":    brID.String,
		"branch_name":  brName.String,
		"role":         roleName,
		"date_from":    dateFrom,
		"date_to":      dateTo,
		"days_present": daysPresent,
		"days_absent":  daysAbsent,
		"total_hours":  math.Round(totalHoursSum*100) / 100,
		"records":      days,
	}, nil
}

// ──────────────────────────────────────────
// Bulk Create from Excel Upload
// ──────────────────────────────────────────

// BulkCreateFromUpload processes parsed Excel records for a branch.
// It clears any previous upload data for the same branch+date, then inserts fresh.
func (s *Store) BulkCreateFromUpload(records []ExcelPunchRecord, branchID string) (*UploadResult, error) {
	if len(records) == 0 {
		return &UploadResult{Details: []UploadRowDetail{}}, nil
	}

	date := records[0].Date.Format("2006-01-02")

	// Remove previously uploaded records for this branch+date (keeps self-punches intact)
	_, err := s.db.Exec(
		`DELETE FROM attendance WHERE branch_id = $1::uuid AND date = $2 AND source = 'upload'`,
		branchID, date,
	)
	if err != nil {
		return nil, fmt.Errorf("clear previous upload: %w", err)
	}

	result := &UploadResult{
		Date:      date,
		TotalRows: len(records),
		Details:   []UploadRowDetail{},
	}

	for _, rec := range records {
		detail := UploadRowDetail{
			Row:          rec.RowNum,
			EmployeeName: rec.Name,
			EmployeeCode: rec.ECode,
		}

		// Find or create the employee
		userID, created, err := s.findOrCreateEmployee(rec.Name, rec.ECode, branchID)
		if err != nil {
			detail.Status = "error"
			detail.Message = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("row %d (%s): %s", rec.RowNum, rec.Name, err.Error()))
			result.Details = append(result.Details, detail)
			continue
		}
		if created {
			result.EmployeesCreated++
			detail.Status = "employee_created"
			detail.Message = "new employee created with default password (defab@123)"
		}

		// No punches → still ensure employee exists, but skip attendance creation
		if len(rec.Punches) == 0 {
			if detail.Status == "" {
				detail.Status = "skipped"
				detail.Message = "no punch times"
			}
			result.Skipped++
			result.Details = append(result.Details, detail)
			continue
		}

		// Insert one attendance row per IN/OUT pair
		punchesCreated := 0
		for seq, p := range rec.Punches {
			if p.In == nil {
				continue // can't record a session without a punch-in time
			}

			sessionSeq := seq + 1
			var punchOut interface{}
			var totalHours float64
			if p.Out != nil {
				punchOut = *p.Out
				totalHours = math.Round(p.Out.Sub(*p.In).Hours()*100) / 100
			}

			notesText := fmt.Sprintf("Dept: %s, Shift: %s", rec.Department, rec.Shift)

			_, err := s.db.Exec(`
				INSERT INTO attendance (user_id, branch_id, date, punch_in, punch_out, total_hours, notes, session_seq, source)
				VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, 'upload')
			`, userID, branchID, date, *p.In, punchOut, totalHours, notesText, sessionSeq)
			if err != nil {
				detail.Message += fmt.Sprintf("; session %d error: %s", sessionSeq, err.Error())
				continue
			}
			punchesCreated++
		}

		detail.PunchesRecorded = punchesCreated
		if punchesCreated > 0 {
			if detail.Status == "" {
				detail.Status = "created"
			}
			result.RecordsCreated += punchesCreated
		} else if detail.Status == "" {
			detail.Status = "skipped"
			detail.Message = "no valid punch-in times"
			result.Skipped++
		}
		result.Details = append(result.Details, detail)
	}

	return result, nil
}

// ──────────────────────────────────────────
// Employee lookup / auto-creation
// ──────────────────────────────────────────

// findOrCreateEmployee matches an employee by code or name,
// or creates a new Employee user with a default password.
func (s *Store) findOrCreateEmployee(name, employeeCode, branchID string) (string, bool, error) {
	// 1. Match by employee_code (most reliable)
	if employeeCode != "" {
		var uid string
		err := s.db.QueryRow(`SELECT id FROM users WHERE employee_code = $1`, employeeCode).Scan(&uid)
		if err == nil {
			return uid, false, nil
		}
		if err != sql.ErrNoRows {
			return "", false, err
		}
	}

	// 2. Match by name (case-insensitive)
	var uid string
	err := s.db.QueryRow(
		`SELECT id FROM users WHERE LOWER(TRIM(name)) = LOWER(TRIM($1)) ORDER BY created_at ASC LIMIT 1`,
		name,
	).Scan(&uid)
	if err == nil {
		// Backfill employee_code if not already set
		if employeeCode != "" {
			s.db.Exec(
				`UPDATE users SET employee_code = $1 WHERE id = $2 AND (employee_code IS NULL OR employee_code = '')`,
				employeeCode, uid,
			)
		}
		return uid, false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}

	// 3. Auto-create as Employee
	var roleID uint
	if err := s.db.QueryRow(`SELECT id FROM roles WHERE name = 'Employee'`).Scan(&roleID); err != nil {
		return "", false, fmt.Errorf("employee role not found: %w", err)
	}

	email := generateEmployeeEmail(name, employeeCode)
	hash, err := auth.HashPassword("defab@123")
	if err != nil {
		return "", false, fmt.Errorf("hash password: %w", err)
	}

	var ecodeParam interface{}
	if employeeCode != "" {
		ecodeParam = employeeCode
	}

	err = s.db.QueryRow(`
		INSERT INTO users (name, email, password_hash, role_id, branch_id, employee_code, is_active)
		VALUES ($1, $2, $3, $4, $5::uuid, $6, TRUE)
		RETURNING id
	`, name, email, hash, roleID, branchID, ecodeParam).Scan(&uid)
	if err != nil {
		// Email collision — retry with a timestamp suffix
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			email = fmt.Sprintf("%s.%d@defab.local", sanitizeName(name), time.Now().UnixNano()%100000)
			err = s.db.QueryRow(`
				INSERT INTO users (name, email, password_hash, role_id, branch_id, employee_code, is_active)
				VALUES ($1, $2, $3, $4, $5::uuid, $6, TRUE)
				RETURNING id
			`, name, email, hash, roleID, branchID, ecodeParam).Scan(&uid)
		}
		if err != nil {
			return "", false, fmt.Errorf("create employee: %w", err)
		}
	}

	return uid, true, nil
}

func generateEmployeeEmail(name, ecode string) string {
	base := sanitizeName(name)
	if ecode != "" {
		return fmt.Sprintf("%s.%s@defab.local", base, ecode)
	}
	return fmt.Sprintf("%s@defab.local", base)
}

func sanitizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", ".")
	var buf strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			buf.WriteRune(r)
		}
	}
	result := buf.String()
	if result == "" {
		result = "employee"
	}
	return result
}

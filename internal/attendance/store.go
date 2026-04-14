package attendance

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

// ──────────────────────────────────────────
// Punch In
// ──────────────────────────────────────────

func (s *Store) PunchIn(userID, branchID, notes string) (map[string]interface{}, error) {
	today := time.Now().Format("2006-01-02")

	// Check if already punched in today
	var existingID string
	var existingOut sql.NullTime
	err := s.db.QueryRow(`SELECT id, punch_out FROM attendance WHERE user_id = $1 AND date = $2`, userID, today).Scan(&existingID, &existingOut)
	if err == nil {
		// Record exists for today
		if !existingOut.Valid {
			return nil, fmt.Errorf("already punched in today")
		}
		// Already punched out — allow re-punch-in: reset punch_in to now, clear punch_out
		var punchIn time.Time
		err = s.db.QueryRow(`
			UPDATE attendance SET punch_in = NOW(), punch_out = NULL
			WHERE id = $1 RETURNING punch_in
		`, existingID).Scan(&punchIn)
		if err != nil {
			return nil, fmt.Errorf("re-punch in: %w", err)
		}
		return map[string]interface{}{
			"id":        existingID,
			"user_id":   userID,
			"date":      today,
			"punch_in":  punchIn,
			"punch_out": nil,
			"notes":     notes,
		}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	var brParam interface{}
	if branchID != "" {
		brParam = branchID
	}

	var id string
	var punchIn time.Time
	err = s.db.QueryRow(`
		INSERT INTO attendance (user_id, branch_id, date, punch_in, notes)
		VALUES ($1, $2, $3, NOW(), $4)
		RETURNING id, punch_in
	`, userID, brParam, today, notes).Scan(&id, &punchIn)
	if err != nil {
		return nil, fmt.Errorf("punch in: %w", err)
	}

	return map[string]interface{}{
		"id":        id,
		"user_id":   userID,
		"date":      today,
		"punch_in":  punchIn,
		"punch_out": nil,
		"notes":     notes,
	}, nil
}

// ──────────────────────────────────────────
// Punch Out
// ──────────────────────────────────────────

func (s *Store) PunchOut(userID, notes string) (map[string]interface{}, error) {
	today := time.Now().Format("2006-01-02")

	var id string
	var punchIn time.Time
	var existingOut sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, punch_in, punch_out FROM attendance WHERE user_id = $1 AND date = $2
	`, userID, today).Scan(&id, &punchIn, &existingOut)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no punch-in found for today")
	}
	if err != nil {
		return nil, err
	}
	if existingOut.Valid {
		return nil, fmt.Errorf("already punched out today")
	}

	now := time.Now()
	sessionHours := math.Round(now.Sub(punchIn).Hours()*100) / 100

	// Accumulate hours from previous sessions today
	var prevHours float64
	s.db.QueryRow(`SELECT total_hours FROM attendance WHERE id = $1`, id).Scan(&prevHours)
	hours := math.Round((prevHours+sessionHours)*100) / 100

	if notes != "" {
		_, err = s.db.Exec(`
			UPDATE attendance SET punch_out = NOW(), total_hours = $1, notes = $2 WHERE id = $3
		`, hours, notes, id)
	} else {
		_, err = s.db.Exec(`
			UPDATE attendance SET punch_out = NOW(), total_hours = $1 WHERE id = $2
		`, hours, id)
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
// List — today's attendance for all users
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

	n++
	datePH := n
	args = append(args, today)

	var total int
	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM users u %s`, where)
	if err := s.db.QueryRow(countQ, args[:len(args)-1]...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	limitP := n
	n++
	offsetP := n
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT u.id, u.name, u.branch_id, COALESCE(b.name,'') AS branch_name,
		       COALESCE(r.name,'') AS role_name,
		       a.id AS att_id, a.punch_in, a.punch_out, COALESCE(a.total_hours,0), COALESCE(a.notes,'')
		FROM users u
		LEFT JOIN branches b ON b.id = u.branch_id
		LEFT JOIN roles r ON r.id = u.role_id
		LEFT JOIN attendance a ON a.user_id = u.id AND a.date = $%d
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
			attID        sql.NullString
			punchIn      sql.NullTime
			punchOut     sql.NullTime
			totalHours   float64
			notes        string
		)
		if err := rows.Scan(&uid, &uname, &brID, &brName, &roleName, &attID, &punchIn, &punchOut, &totalHours, &notes); err != nil {
			return nil, 0, err
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
			"total_hours": 0.0,
			"notes":       "",
			"status":      "absent",
		}
		if attID.Valid {
			row["status"] = "present"
			row["total_hours"] = totalHours
			row["notes"] = notes
			if punchIn.Valid {
				row["punch_in"] = punchIn.Time
			}
			if punchOut.Valid {
				row["punch_out"] = punchOut.Time
			}
		}
		list = append(list, row)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID — date-range attendance for a user
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

	// Fetch attendance records in range
	rows, err := s.db.Query(`
		SELECT date::text, punch_in, punch_out, total_hours, notes
		FROM attendance
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date
	`, targetUserID, dateFrom, dateTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build map of date -> record
	presentMap := map[string]map[string]interface{}{}
	for rows.Next() {
		var date, notes string
		var punchIn time.Time
		var punchOut sql.NullTime
		var totalHours float64
		if err := rows.Scan(&date, &punchIn, &punchOut, &totalHours, &notes); err != nil {
			continue
		}
		rec := map[string]interface{}{
			"punch_in":    punchIn,
			"punch_out":   nil,
			"total_hours": totalHours,
			"notes":       notes,
		}
		if punchOut.Valid {
			rec["punch_out"] = punchOut.Time
		}
		presentMap[date] = rec
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
			"punch_in":    nil,
			"punch_out":   nil,
			"total_hours": 0.0,
			"notes":       "",
		}
		if rec, ok := presentMap[ds]; ok {
			day["status"] = "present"
			day["punch_in"] = rec["punch_in"]
			day["punch_out"] = rec["punch_out"]
			day["total_hours"] = rec["total_hours"]
			day["notes"] = rec["notes"]
			totalHoursSum += rec["total_hours"].(float64)
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

package attendance

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
	"github.com/xuri/excelize/v2"
)

// ExcelPunchRecord represents one employee's attendance data parsed from an Excel row.
type ExcelPunchRecord struct {
	RowNum     int
	ECode      string
	Name       string
	Department string
	Shift      string
	Date       time.Time
	Punches    []PunchPair
}

// PunchPair represents a single IN/OUT punch pair.
type PunchPair struct {
	In  *time.Time
	Out *time.Time
}

// parseAttendanceExcel parses a biometric "Daily Attendance IN/OUT Punch Report".
// Supports both .xls (BIFF) and .xlsx formats.
func parseAttendanceExcel(fh *multipart.FileHeader) ([]ExcelPunchRecord, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))

	var rows [][]string
	var err error

	switch ext {
	case ".xlsx":
		rows, err = readXLSX(fh)
	case ".xls":
		rows, err = readXLS(fh)
	default:
		return nil, fmt.Errorf("unsupported format %q; upload .xls or .xlsx", ext)
	}
	if err != nil {
		return nil, err
	}

	return parseRows(rows)
}

// readXLSX reads an .xlsx file into a 2D string grid.
func readXLSX(fh *multipart.FileHeader) ([][]string, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	f, err := excelize.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read sheet: %w", err)
	}
	return rows, nil
}

// readXLS reads a legacy .xls (BIFF) file into a 2D string grid.
func readXLS(fh *multipart.FileHeader) ([][]string, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "attendance-*.xls")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	wb, err := xls.OpenFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read xls: %w", err)
	}

	sheet, err := wb.GetSheet(0)
	if err != nil {
		return nil, fmt.Errorf("get sheet: %w", err)
	}

	var rows [][]string
	for r := 0; r < sheet.GetNumberRows(); r++ {
		row, err := sheet.GetRow(r)
		if err != nil {
			rows = append(rows, []string{})
			continue
		}
		var cells []string
		for c := 0; c < len(row.GetCols()); c++ {
			cell, err := row.GetCol(c)
			if err != nil {
				cells = append(cells, "")
				continue
			}
			cells = append(cells, strings.TrimSpace(cell.GetString()))
		}
		rows = append(rows, cells)
	}

	return rows, nil
}

// parseRows takes a 2D string grid and produces ExcelPunchRecords.
func parseRows(rows [][]string) ([]ExcelPunchRecord, error) {
	// 1. Find the report date from header area
	reportDate, err := findReportDate(rows)
	if err != nil {
		return nil, err
	}

	// 2. Locate header row and map column names → indices
	headerIdx, colMap, err := findHeaders(rows)
	if err != nil {
		return nil, err
	}

	nameCol, ok := colMap["Name"]
	if !ok {
		return nil, fmt.Errorf("'Name' column not found in headers")
	}
	ecodeCol := colIdx(colMap, "E. Code")
	deptCol := colIdx(colMap, "Department")
	shiftCol := colIdx(colMap, "Shift")

	// Collect IN/OUT column pairs (IN-1/OUT-1 … IN-5/OUT-5)
	type inOutPair struct{ in, out int }
	var punchCols []inOutPair
	for i := 1; i <= 5; i++ {
		inKey := fmt.Sprintf("IN-%d", i)
		if inIdx, ok := colMap[inKey]; ok {
			outIdx := -1
			if oi, ok2 := colMap[fmt.Sprintf("OUT-%d", i)]; ok2 {
				outIdx = oi
			}
			punchCols = append(punchCols, inOutPair{inIdx, outIdx})
		}
	}

	// 3. Parse data rows
	var records []ExcelPunchRecord
	for i := headerIdx + 1; i < len(rows); i++ {
		row := rows[i]
		name := strings.TrimSpace(cellVal(row, nameCol))
		if name == "" {
			continue
		}

		rec := ExcelPunchRecord{
			RowNum:     i + 1, // 1-indexed for display
			ECode:      strings.TrimSpace(cellVal(row, ecodeCol)),
			Name:       name,
			Department: strings.TrimSpace(cellVal(row, deptCol)),
			Shift:      strings.TrimSpace(cellVal(row, shiftCol)),
			Date:       reportDate,
		}

		for _, pc := range punchCols {
			inStr := strings.TrimSpace(cellVal(row, pc.in))
			outStr := ""
			if pc.out >= 0 {
				outStr = strings.TrimSpace(cellVal(row, pc.out))
			}
			if inStr == "" && outStr == "" {
				continue
			}
			pair := PunchPair{}
			if inStr != "" {
				t := timeOfDay(reportDate, inStr)
				if !t.IsZero() {
					pair.In = &t
				}
			}
			if outStr != "" {
				t := timeOfDay(reportDate, outStr)
				if !t.IsZero() {
					pair.Out = &t
				}
			}
			if pair.In != nil || pair.Out != nil {
				rec.Punches = append(rec.Punches, pair)
			}
		}

		records = append(records, rec)
	}

	return records, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

// findReportDate scans the first 10 rows for the attendance report date.
func findReportDate(rows [][]string) (time.Time, error) {
	for i := 0; i < len(rows) && i < 12; i++ {
		row := rows[i]
		for j, cell := range row {
			cell = strings.TrimSpace(cell)

			// "Date   16-April-2026" in one cell
			if strings.HasPrefix(cell, "Date") {
				dateStr := strings.TrimSpace(strings.TrimPrefix(cell, "Date"))
				if t, ok := tryParseDate(dateStr); ok {
					return t, nil
				}
			}

			// "Date" in one cell, date value in subsequent cell(s)
			if cell == "Date" {
				for k := j + 1; k < len(row) && k < j+6; k++ {
					v := strings.TrimSpace(cellVal(row, k))
					if t, ok := tryParseDate(v); ok {
						return t, nil
					}
				}
			}

			// Date range: "Apr 16 2026  To  Apr 16 2026"
			if idx := strings.Index(strings.ToLower(cell), " to "); idx > 0 {
				if t, ok := tryParseDate(strings.TrimSpace(cell[:idx])); ok {
					return t, nil
				}
			}

			// Standalone date value
			if t, ok := tryParseDate(cell); ok {
				// Avoid matching short numeric strings
				if len(cell) > 6 {
					return t, nil
				}
			}
		}
	}
	return time.Time{}, fmt.Errorf("could not find report date in the Excel file")
}

var dateLayouts = []string{
	"2-January-2006",
	"02-January-2006",
	"2-Jan-2006",
	"02-Jan-2006",
	"Jan 2 2006",
	"Jan 02 2006",
	"January 2 2006",
	"January 02 2006",
	"Jan 2, 2006",
	"2006-01-02",
	"01/02/2006",
	"02/01/2006",
}

func tryParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// findHeaders scans rows for the header row containing "SN" or "SNo" and returns column mapping.
func findHeaders(rows [][]string) (int, map[string]int, error) {
	for i, row := range rows {
		for _, cell := range row {
			trimmed := strings.TrimSpace(cell)
			if trimmed == "SN" || trimmed == "SNo" {
				colMap := map[string]int{}
				for j, h := range row {
					h = strings.TrimSpace(h)
					if h != "" {
						// Normalize variants
						if h == "SNo" {
							h = "SN"
						}
						colMap[h] = j
					}
				}
				return i, colMap, nil
			}
		}
	}
	return 0, nil, fmt.Errorf("header row not found — expected a row containing 'SN' or 'SNo'")
}

func colIdx(m map[string]int, key string) int {
	if v, ok := m[key]; ok {
		return v
	}
	return -1
}

func cellVal(row []string, col int) string {
	if col < 0 || col >= len(row) {
		return ""
	}
	return row[col]
}

// timeOfDay parses "HH:MM" and combines it with the report date.
func timeOfDay(date time.Time, s string) time.Time {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return time.Time{}
	}
	var h, m int
	if _, err := fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return time.Time{}
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return time.Time{}
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.Local)
}

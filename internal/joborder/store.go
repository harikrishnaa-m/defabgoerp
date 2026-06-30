package joborder

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// ──────────────────────────────────────────
// Create
// ──────────────────────────────────────────

func (s *Store) CreateJobOrder(in CreateJobOrderInput, userID, branchID, warehouseID string) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if in.MaterialSource == "" {
		in.MaterialSource = MaterialSourceCustomer
	}
	if in.MaterialSource != MaterialSourceCustomer && in.MaterialSource != MaterialSourceStore {
		return "", fmt.Errorf("invalid material_source: %s", in.MaterialSource)
	}

	// Find or create customer
	if in.CustomerID == "" && in.CustomerPhone != "" {
		err = tx.QueryRow(`SELECT id FROM customers WHERE phone = $1`, in.CustomerPhone).Scan(&in.CustomerID)
		if err == sql.ErrNoRows {
			var maxCode sql.NullString
			tx.QueryRow(`SELECT MAX(customer_code) FROM customers WHERE customer_code LIKE 'CUS%'`).Scan(&maxCode)
			next := 1
			if maxCode.Valid && len(maxCode.String) > 3 {
				fmt.Sscanf(maxCode.String[3:], "%d", &next)
				next++
			}
			code := fmt.Sprintf("CUS%04d", next)
			err = tx.QueryRow(`
				INSERT INTO customers (customer_code, name, phone, email)
				VALUES ($1, $2, $3, $4)
				RETURNING id
			`, code, in.CustomerName, in.CustomerPhone, in.CustomerEmail).Scan(&in.CustomerID)
			if err != nil {
				return "", fmt.Errorf("create customer: %w", err)
			}
		} else if err != nil {
			return "", fmt.Errorf("find customer: %w", err)
		}
	}

	jobNumber := s.nextJobNumber(tx)

	var branchParam, whParam interface{}
	if branchID != "" {
		branchParam = branchID
	}
	if warehouseID != "" {
		whParam = warehouseID
	}

	var jobID string

	// Resolve received date: use provided date or fall back to NOW()
	receivedDate := "NOW()"
	if in.ReceivedDate != "" {
		receivedDate = "'" + in.ReceivedDate + "'::timestamptz"
	}

	err = tx.QueryRow(fmt.Sprintf(`
		INSERT INTO job_orders
			(job_number, customer_id, branch_id, warehouse_id, job_type, material_source,
			 status, payment_status, received_date, expected_delivery_date,
			 sub_amount, discount_amount, gst_amount, net_amount,
			 notes, sample_provided, sample_description, measurement_bill_number,
			 image_url, design_image_url, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,'RECEIVED','UNPAID',%s,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id
	`, receivedDate), jobNumber, in.CustomerID, branchParam, whParam, in.JobType, in.MaterialSource,
		in.ExpectedDeliveryDate,
		round2(in.SubAmount), round2(in.DiscountAmount), round2(in.GSTAmount), round2(in.NetAmount),
		in.Notes, in.SampleProvided, in.SampleDescription, in.MeasurementBillNumber,
		nilIfEmpty(in.ImageURL), nilIfEmpty(in.DesignImageURL), userID).Scan(&jobID)
	if err != nil {
		return "", fmt.Errorf("create job order: %w", err)
	}

	// Insert initial status history entry
	_, err = tx.Exec(`
		INSERT INTO job_order_status_history (job_order_id, status, notes, updated_by)
		VALUES ($1, 'RECEIVED', $2, $3)
	`, jobID, "Job order created", userID)
	if err != nil {
		return "", fmt.Errorf("insert initial status: %w", err)
	}

	// Insert items
	for _, item := range in.Items {
		piecesJSON, _ := json.Marshal(item.Pieces)
		_, err = tx.Exec(`
			INSERT INTO job_order_items
				(job_order_id, category, sub_category, pieces,
				 quantity, unit_price, discount, tax_percent, cgst, sgst, total_price)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		`, jobID, item.Category, item.SubCategory, string(piecesJSON),
			item.Quantity, round2(item.UnitPrice), round2(item.Discount),
			item.TaxPercent, round2(item.CGST), round2(item.SGST), round2(item.TotalPrice))
		if err != nil {
			return "", fmt.Errorf("insert job order item: %w", err)
		}
	}

	// Insert materials & deduct stock (only for STORE source)
	if in.MaterialSource == MaterialSourceStore {
		for _, mat := range in.Materials {
			if mat.QuantityUsed <= 0 {
				continue
			}

			if mat.VariantID != "" {
				// Variant-based path: use the provided variant_id directly
				variantID := mat.VariantID
				whID := mat.WarehouseID
				_, err = tx.Exec(`
					INSERT INTO job_order_materials (job_order_id, variant_id, variant_warehouse_id, quantity_used)
					VALUES ($1,$2,$3,$4)
				`, jobID, nilIfEmpty(variantID), nilIfEmpty(whID), mat.QuantityUsed)
				if err != nil {
					return "", fmt.Errorf("insert job material: %w", err)
				}
				res, err := tx.Exec(`
					UPDATE stocks SET quantity = quantity - $1, updated_at = NOW()
					WHERE variant_id = $2 AND warehouse_id = $3 AND quantity >= $1
				`, mat.QuantityUsed, variantID, whID)
				if err != nil {
					return "", fmt.Errorf("deduct variant stock: %w", err)
				}
				rows, _ := res.RowsAffected()
				if rows == 0 {
					return "", fmt.Errorf("insufficient stock for variant %s", mat.VariantID)
				}
				_, err = tx.Exec(`
					INSERT INTO stock_movements
						(variant_id, from_warehouse_id, quantity, movement_type, reference, status)
					VALUES ($1,$2,$3,'JOB_OUT',$4,'COMPLETED')
				`, variantID, whID, mat.QuantityUsed, "JOB:"+jobNumber)
				if err != nil {
					return "", fmt.Errorf("insert stock_movements: %w", err)
				}
			} else {
				// Legacy raw_material_stocks path
				_, err = tx.Exec(`
					INSERT INTO job_order_materials (job_order_id, raw_material_stock_id, quantity_used)
					VALUES ($1,$2,$3)
				`, jobID, mat.RawMaterialStockID, mat.QuantityUsed)
				if err != nil {
					return "", fmt.Errorf("insert job material: %w", err)
				}

				var itemName, rmWhID string
				err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, mat.RawMaterialStockID).Scan(&itemName, &rmWhID)
				if err != nil {
					return "", fmt.Errorf("raw material stock not found: %w", err)
				}
				res, err := tx.Exec(`
					UPDATE raw_material_stocks SET quantity = quantity - $1, updated_at = NOW()
					WHERE id = $2 AND quantity >= $1
				`, mat.QuantityUsed, mat.RawMaterialStockID)
				if err != nil {
					return "", fmt.Errorf("deduct raw material stock: %w", err)
				}
				rows, _ := res.RowsAffected()
				if rows == 0 {
					return "", fmt.Errorf("insufficient raw material stock for %s", itemName)
				}
				_, err = tx.Exec(`
					INSERT INTO raw_material_movements
						(item_name, warehouse_id, quantity, movement_type, reference)
					VALUES ($1,$2,$3,'OUT',$4)
				`, itemName, rmWhID, mat.QuantityUsed, "JOB:"+jobNumber)
				if err != nil {
					return "", fmt.Errorf("create raw material movement: %w", err)
				}
			}
		}
	}

	// Insert payments
	var totalPaid float64
	for _, p := range in.Payments {
		amt := round2(p.Amount)
		_, err = tx.Exec(`
			INSERT INTO job_order_payments (job_order_id, amount, payment_method, reference)
			VALUES ($1,$2,$3,$4)
		`, jobID, amt, p.PaymentMethod, p.Reference)
		if err != nil {
			return "", fmt.Errorf("insert payment: %w", err)
		}
		totalPaid += amt
	}

	// Update payment status
	ps := "UNPAID"
	if totalPaid >= in.NetAmount && in.NetAmount > 0 {
		ps = "PAID"
	} else if totalPaid > 0 {
		ps = "PARTIAL"
	}
	if ps != "UNPAID" {
		_, err = tx.Exec(`UPDATE job_orders SET payment_status = $1 WHERE id = $2`, ps, jobID)
		if err != nil {
			return "", fmt.Errorf("update payment status: %w", err)
		}
	}

	// Update customer total_purchases
	_, err = tx.Exec(`
		UPDATE customers
		SET total_purchases = total_purchases + $1, updated_at = NOW()
		WHERE id = $2
	`, in.NetAmount, in.CustomerID)
	if err != nil {
		return "", fmt.Errorf("update customer total_purchases: %w", err)
	}

	// Auto-create job invoice
	invNum := s.nextInvoiceNumber(tx)
	_, err = tx.Exec(`
		INSERT INTO job_invoices
			(invoice_number, job_order_id, branch_id, customer_id,
			 sub_amount, discount_amount, gst_amount, net_amount, payment_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, invNum, jobID, branchParam, in.CustomerID,
		round2(in.SubAmount), round2(in.DiscountAmount), round2(in.GSTAmount), round2(in.NetAmount), ps)
	if err != nil {
		return "", fmt.Errorf("create job invoice: %w", err)
	}

	return jobID, tx.Commit()
}

// ──────────────────────────────────────────
// Update header
// ──────────────────────────────────────────

func (s *Store) UpdateJobOrder(id string, in UpdateJobOrderInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Find-or-create customer by phone if customer_id not provided
	if (in.CustomerID == nil || *in.CustomerID == "") && in.CustomerPhone != "" {
		var custID string
		err = tx.QueryRow(`SELECT id FROM customers WHERE phone = $1`, in.CustomerPhone).Scan(&custID)
		if err == sql.ErrNoRows {
			var maxCode sql.NullString
			tx.QueryRow(`SELECT MAX(code) FROM customers WHERE code LIKE 'CUS%'`).Scan(&maxCode)
			next := 1
			if maxCode.Valid && len(maxCode.String) > 3 {
				fmt.Sscanf(maxCode.String[3:], "%d", &next)
				next++
			}
			code := fmt.Sprintf("CUS%04d", next)
			err = tx.QueryRow(`
				INSERT INTO customers (code, name, phone, email)
				VALUES ($1,$2,$3,$4) RETURNING id
			`, code, in.CustomerName, in.CustomerPhone, in.CustomerEmail).Scan(&custID)
			if err != nil {
				return fmt.Errorf("create customer: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("customer lookup: %w", err)
		}
		in.CustomerID = &custID
	}

	// Build dynamic header update
	q := `UPDATE job_orders SET updated_at = NOW()`
	args := []interface{}{}
	n := 0

	if in.CustomerID != nil {
		n++
		q += fmt.Sprintf(", customer_id = $%d", n)
		args = append(args, *in.CustomerID)
	}
	if in.JobType != nil {
		n++
		q += fmt.Sprintf(", job_type = $%d", n)
		args = append(args, *in.JobType)
	}
	if in.MaterialSource != nil {
		n++
		q += fmt.Sprintf(", material_source = $%d", n)
		args = append(args, *in.MaterialSource)
	}
	if in.ExpectedDeliveryDate != nil {
		n++
		q += fmt.Sprintf(", expected_delivery_date = $%d", n)
		args = append(args, *in.ExpectedDeliveryDate)
	}
	if in.Notes != nil {
		n++
		q += fmt.Sprintf(", notes = $%d", n)
		args = append(args, *in.Notes)
	}
	if in.SampleProvided != nil {
		n++
		q += fmt.Sprintf(", sample_provided = $%d", n)
		args = append(args, *in.SampleProvided)
	}
	if in.SampleDescription != nil {
		n++
		q += fmt.Sprintf(", sample_description = $%d", n)
		args = append(args, *in.SampleDescription)
	}
	if in.MeasurementBillNumber != nil {
		n++
		q += fmt.Sprintf(", measurement_bill_number = $%d", n)
		args = append(args, *in.MeasurementBillNumber)
	}
	if in.ImageURL != nil {
		n++
		q += fmt.Sprintf(", image_url = $%d", n)
		args = append(args, nilIfEmpty(*in.ImageURL))
	}
	if in.DesignImageURL != nil {
		n++
		q += fmt.Sprintf(", design_image_url = $%d", n)
		args = append(args, nilIfEmpty(*in.DesignImageURL))
	}
	if in.SubAmount != nil {
		n++
		q += fmt.Sprintf(", sub_amount = $%d", n)
		args = append(args, round2(*in.SubAmount))
	}
	if in.DiscountAmount != nil {
		n++
		q += fmt.Sprintf(", discount_amount = $%d", n)
		args = append(args, round2(*in.DiscountAmount))
	}
	if in.GSTAmount != nil {
		n++
		q += fmt.Sprintf(", gst_amount = $%d", n)
		args = append(args, round2(*in.GSTAmount))
	}

	// Track old net_amount for customer total_purchases adjustment
	var oldNetAmount float64
	var oldCustomerID string
	if in.NetAmount != nil {
		err = tx.QueryRow(`SELECT customer_id, net_amount FROM job_orders WHERE id = $1`, id).Scan(&oldCustomerID, &oldNetAmount)
		if err != nil {
			return fmt.Errorf("fetch old net_amount: %w", err)
		}
		n++
		q += fmt.Sprintf(", net_amount = $%d", n)
		args = append(args, round2(*in.NetAmount))
	}

	if n > 0 {
		n++
		q += fmt.Sprintf(" WHERE id = $%d", n)
		args = append(args, id)

		res, err := tx.Exec(q, args...)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			return sql.ErrNoRows
		}
	}

	// Update customer total_purchases if net_amount changed
	if in.NetAmount != nil {
		diff := round2(*in.NetAmount) - oldNetAmount
		if diff != 0 {
			custID := oldCustomerID
			if in.CustomerID != nil {
				custID = *in.CustomerID
			}
			_, err = tx.Exec(`
				UPDATE customers SET total_purchases = total_purchases + $1, updated_at = NOW()
				WHERE id = $2
			`, diff, custID)
			if err != nil {
				return fmt.Errorf("adjust customer total_purchases: %w", err)
			}
		}
	}

	// Replace items if provided
	if len(in.Items) > 0 {
		_, err = tx.Exec(`DELETE FROM job_order_items WHERE job_order_id = $1`, id)
		if err != nil {
			return fmt.Errorf("delete old items: %w", err)
		}
		for _, it := range in.Items {
			piecesJSON, _ := json.Marshal(it.Pieces)
			_, err = tx.Exec(`
				INSERT INTO job_order_items
					(job_order_id, category, sub_category, pieces,
					 quantity, unit_price, discount, tax_percent, cgst, sgst, total_price)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			`, id, it.Category, it.SubCategory, string(piecesJSON),
				it.Quantity, round2(it.UnitPrice), round2(it.Discount),
				it.TaxPercent, round2(it.CGST), round2(it.SGST), round2(it.TotalPrice))
			if err != nil {
				return fmt.Errorf("insert item: %w", err)
			}
		}
	}

	// Replace materials if provided (reverse old stock, deduct new)
	if len(in.Materials) > 0 {
		// Get current material_source
		var matSrc string
		err = tx.QueryRow(`SELECT material_source FROM job_orders WHERE id = $1`, id).Scan(&matSrc)
		if err != nil {
			return fmt.Errorf("fetch material_source: %w", err)
		}
		if in.MaterialSource != nil {
			matSrc = *in.MaterialSource
		}

		// Reverse old materials if they were from STORE
		var oldMatSrc string
		tx.QueryRow(`SELECT material_source FROM job_orders WHERE id = $1`, id).Scan(&oldMatSrc)

		oldRows, err := tx.Query(`
			SELECT COALESCE(raw_material_stock_id::text,''), COALESCE(variant_id::text,''),
			       COALESCE(variant_warehouse_id::text,''), quantity_used
			FROM job_order_materials WHERE job_order_id = $1`, id)
		if err != nil {
			return fmt.Errorf("fetch old materials: %w", err)
		}
		type matItem struct {
			stockID   string
			variantID string
			whID      string
			qty       float64
		}
		var oldMats []matItem
		for oldRows.Next() {
			var m matItem
			if err := oldRows.Scan(&m.stockID, &m.variantID, &m.whID, &m.qty); err != nil {
				oldRows.Close()
				return err
			}
			oldMats = append(oldMats, m)
		}
		oldRows.Close()

		// Reverse old stock if it was STORE-sourced
		if oldMatSrc == MaterialSourceStore {
			for _, m := range oldMats {
				if m.variantID != "" {
					_, err = tx.Exec(`UPDATE stocks SET quantity = quantity + $1, updated_at = NOW() WHERE variant_id = $2 AND warehouse_id = $3`, m.qty, m.variantID, m.whID)
					if err != nil {
						return err
					}
					_, err = tx.Exec(`INSERT INTO stock_movements (variant_id, to_warehouse_id, quantity, movement_type, reference, status) VALUES ($1,$2,$3,'JOB_IN',$4,'COMPLETED')`,
						m.variantID, m.whID, m.qty, "JOB_UPDATE_REVERSE:"+id)
					if err != nil {
						return err
					}
				} else {
					var itemName, rmWhID string
					err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, m.stockID).Scan(&itemName, &rmWhID)
					if err != nil {
						return fmt.Errorf("raw material lookup: %w", err)
					}
					_, err = tx.Exec(`UPDATE raw_material_stocks SET quantity = quantity + $1, updated_at = NOW() WHERE id = $2`, m.qty, m.stockID)
					if err != nil {
						return err
					}
					_, err = tx.Exec(`INSERT INTO raw_material_movements (item_name, warehouse_id, quantity, movement_type, reference) VALUES ($1,$2,$3,'IN',$4)`,
						itemName, rmWhID, m.qty, "JOB_UPDATE_REVERSE:"+id)
					if err != nil {
						return err
					}
				}
			}
		}

		// Delete old materials
		_, err = tx.Exec(`DELETE FROM job_order_materials WHERE job_order_id = $1`, id)
		if err != nil {
			return fmt.Errorf("delete old materials: %w", err)
		}

		// Insert new materials and deduct stock if STORE
		for _, m := range in.Materials {
			if m.VariantID != "" {
				variantID := m.VariantID
				whID := m.WarehouseID
				_, err = tx.Exec(`
					INSERT INTO job_order_materials (job_order_id, variant_id, variant_warehouse_id, quantity_used)
					VALUES ($1,$2,$3,$4)
				`, id, nilIfEmpty(variantID), nilIfEmpty(whID), m.QuantityUsed)
				if err != nil {
					return fmt.Errorf("insert material: %w", err)
				}
				if matSrc == MaterialSourceStore {
					_, err = tx.Exec(`UPDATE stocks SET quantity = quantity - $1, updated_at = NOW() WHERE variant_id = $2 AND warehouse_id = $3`, m.QuantityUsed, variantID, whID)
					if err != nil {
						return err
					}
					_, err = tx.Exec(`INSERT INTO stock_movements (variant_id, from_warehouse_id, quantity, movement_type, reference, status) VALUES ($1,$2,$3,'JOB_OUT',$4,'COMPLETED')`,
						variantID, whID, m.QuantityUsed, "JOB_UPDATE:"+id)
					if err != nil {
						return err
					}
				}
			} else {
				_, err = tx.Exec(`
					INSERT INTO job_order_materials (job_order_id, raw_material_stock_id, quantity_used)
					VALUES ($1,$2,$3)
				`, id, m.RawMaterialStockID, m.QuantityUsed)
				if err != nil {
					return fmt.Errorf("insert material: %w", err)
				}

				if matSrc == MaterialSourceStore {
					var itemName, rmWhID string
					err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, m.RawMaterialStockID).Scan(&itemName, &rmWhID)
					if err != nil {
						return fmt.Errorf("raw material lookup: %w", err)
					}
					_, err = tx.Exec(`UPDATE raw_material_stocks SET quantity = quantity - $1, updated_at = NOW() WHERE id = $2`, m.QuantityUsed, m.RawMaterialStockID)
					if err != nil {
						return err
					}
					_, err = tx.Exec(`INSERT INTO raw_material_movements (item_name, warehouse_id, quantity, movement_type, reference) VALUES ($1,$2,$3,'OUT',$4)`,
						itemName, rmWhID, m.QuantityUsed, "JOB_UPDATE:"+id)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// Push status
// ──────────────────────────────────────────

func (s *Store) PushStatus(jobID string, in StatusUpdateInput, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO job_order_status_history (job_order_id, status, notes, updated_by)
		VALUES ($1,$2,$3,$4)
	`, jobID, in.Status, in.Notes, userID)
	if err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}

	deliveredClause := ""
	if in.Status == "DELIVERED" {
		deliveredClause = ", actual_delivery_date = NOW()"
	}

	_, err = tx.Exec(fmt.Sprintf(`
		UPDATE job_orders SET status = $1, updated_at = NOW()%s WHERE id = $2
	`, deliveredClause), in.Status, jobID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// Add payment
// ──────────────────────────────────────────

func (s *Store) AddPayment(jobID string, in PaymentInput) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	amt := round2(in.Amount)

	// Check balance due before accepting payment
	var totalPaid, netAmount float64
	err = tx.QueryRow(`
		SELECT COALESCE(SUM(amount),0) FROM job_order_payments WHERE job_order_id = $1
	`, jobID).Scan(&totalPaid)
	if err != nil {
		return err
	}
	err = tx.QueryRow(`SELECT net_amount FROM job_orders WHERE id = $1`, jobID).Scan(&netAmount)
	if err != nil {
		return err
	}
	balanceDue := round2(netAmount - totalPaid)
	if amt > balanceDue {
		return fmt.Errorf("payment amount %.2f exceeds balance due %.2f", amt, balanceDue)
	}

	_, err = tx.Exec(`
		INSERT INTO job_order_payments (job_order_id, amount, payment_method, reference)
		VALUES ($1,$2,$3,$4)
	`, jobID, amt, in.PaymentMethod, in.Reference)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}

	// Recalculate payment status
	totalPaid += amt
	ps := "UNPAID"
	if totalPaid >= netAmount && netAmount > 0 {
		ps = "PAID"
	} else if totalPaid > 0 {
		ps = "PARTIAL"
	}
	_, err = tx.Exec(`UPDATE job_orders SET payment_status = $1, updated_at = NOW() WHERE id = $2`, ps, jobID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// List
// ──────────────────────────────────────────

func (s *Store) List(branchID *string, status, jobType, search string, limit, offset int) ([]map[string]interface{}, int, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	n := 0

	if branchID != nil && *branchID != "" {
		n++
		where += fmt.Sprintf(" AND jo.branch_id = $%d", n)
		args = append(args, *branchID)
	}
	if status != "" {
		n++
		where += fmt.Sprintf(" AND jo.status = $%d", n)
		args = append(args, status)
	}
	if jobType != "" {
		n++
		where += fmt.Sprintf(" AND jo.job_type = $%d", n)
		args = append(args, jobType)
	}
	if search != "" {
		n++
		where += fmt.Sprintf(" AND (jo.job_number ILIKE $%d OR c.name ILIKE $%d OR c.phone ILIKE $%d)", n, n, n)
		args = append(args, "%"+search+"%")
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*) FROM job_orders jo
		LEFT JOIN customers c ON c.id = jo.customer_id
		%s`, where)
	if err := s.db.QueryRow(countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n++
	limitP := n
	n++
	offsetP := n
	args = append(args, limit, offset)

	q := fmt.Sprintf(`
		SELECT jo.id, jo.job_number, jo.customer_id, jo.branch_id, jo.warehouse_id,
		       jo.job_type, jo.material_source, jo.status, jo.payment_status,
		       jo.received_date, jo.expected_delivery_date, jo.actual_delivery_date,
		       jo.sub_amount, jo.discount_amount, jo.gst_amount, jo.net_amount,
		       jo.notes, jo.sample_provided, jo.sample_description, jo.measurement_bill_number,
		       COALESCE(jo.image_url, '') AS image_url,
		       COALESCE(jo.design_image_url, '') AS design_image_url,
		       jo.created_by, jo.created_at,
		       c.name AS customer_name, c.phone AS customer_phone,
		       COALESCE(b.name, '') AS branch_name,
		       COALESCE(u.name, '') AS created_by_name,
		       COALESCE((SELECT ji.invoice_number FROM job_invoices ji WHERE ji.job_order_id = jo.id LIMIT 1), '') AS invoice_number
		FROM job_orders jo
		LEFT JOIN customers c ON c.id = jo.customer_id
		LEFT JOIN branches b ON b.id = jo.branch_id
		LEFT JOIN users u ON u.id = jo.created_by
		%s
		ORDER BY jo.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitP, offsetP)

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var (
			id, jobNum, custID, jobTyp, matSrc, st, paySt string
			subAmt, discAmt, gstAmt, netAmt               float64
			notes, sampleDesc, measBillNum                string
			sampleProvided                                bool
			createdBy, custName, custPhone                string
			branchName, createdByName                     string
			branchIDVal, whIDVal                          sql.NullString
			imageURL, designImageURL                      string
			invoiceNumber                                 string
			expectedDate                                  sql.NullString
			actualDate                                    sql.NullTime
			receivedDate, createdAt                       sql.NullTime
		)
		if err := rows.Scan(&id, &jobNum, &custID, &branchIDVal, &whIDVal,
			&jobTyp, &matSrc, &st, &paySt,
			&receivedDate, &expectedDate, &actualDate,
			&subAmt, &discAmt, &gstAmt, &netAmt,
			&notes, &sampleProvided, &sampleDesc, &measBillNum,
			&imageURL, &designImageURL,
			&createdBy, &createdAt,
			&custName, &custPhone, &branchName, &createdByName,
			&invoiceNumber); err != nil {
			return nil, 0, err
		}
		item := map[string]interface{}{
			"id":                      id,
			"job_number":              jobNum,
			"customer_id":             custID,
			"customer_name":           custName,
			"customer_phone":          custPhone,
			"branch_id":               branchIDVal.String,
			"branch_name":             branchName,
			"warehouse_id":            whIDVal.String,
			"job_type":                jobTyp,
			"material_source":         matSrc,
			"status":                  st,
			"payment_status":          paySt,
			"received_date":           receivedDate.Time,
			"expected_delivery_date":  expectedDate.String,
			"actual_delivery_date":    nil,
			"sub_amount":              subAmt,
			"discount_amount":         discAmt,
			"gst_amount":              gstAmt,
			"net_amount":              netAmt,
			"notes":                   notes,
			"sample_provided":         sampleProvided,
			"sample_description":      sampleDesc,
			"measurement_bill_number": measBillNum,
			"image_url":               imageURL,
			"design_image_url":        designImageURL,
			"invoice_number":          invoiceNumber,
			"created_by":              createdBy,
			"created_by_name":         createdByName,
			"created_at":              createdAt.Time,
		}
		if actualDate.Valid {
			item["actual_delivery_date"] = actualDate.Time
		}
		list = append(list, item)
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, total, nil
}

// ──────────────────────────────────────────
// GetByID — full detail
// ──────────────────────────────────────────

func (s *Store) GetByID(id string) (map[string]interface{}, error) {
	var (
		jobID, jobNum, custID, jobTyp, matSrc, st, paySt string
		subAmt, discAmt, gstAmt, netAmt                  float64
		notes, sampleDesc, measBillNum                   string
		sampleProvided                                   bool
		createdBy                                        string
		branchIDVal, whIDVal                             sql.NullString
		imageURL, designImageURL                         sql.NullString
		expectedDate                                     sql.NullString
		actualDate                                       sql.NullTime
		receivedDate, createdAt, updatedAt               sql.NullTime
	)
	err := s.db.QueryRow(`
		SELECT id, job_number, customer_id, branch_id, warehouse_id,
		       job_type, material_source, status, payment_status,
		       received_date, expected_delivery_date, actual_delivery_date,
		       sub_amount, discount_amount, gst_amount, net_amount,
		       notes, sample_provided, sample_description, measurement_bill_number,
		       COALESCE(image_url, '') AS image_url,
		       COALESCE(design_image_url, '') AS design_image_url,
		       created_by, created_at, updated_at
		FROM job_orders WHERE id = $1
	`, id).Scan(&jobID, &jobNum, &custID, &branchIDVal, &whIDVal,
		&jobTyp, &matSrc, &st, &paySt,
		&receivedDate, &expectedDate, &actualDate,
		&subAmt, &discAmt, &gstAmt, &netAmt,
		&notes, &sampleProvided, &sampleDesc, &measBillNum,
		&imageURL, &designImageURL,
		&createdBy, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"id":                      jobID,
		"job_number":              jobNum,
		"customer_id":             custID,
		"branch_id":               branchIDVal.String,
		"warehouse_id":            whIDVal.String,
		"job_type":                jobTyp,
		"material_source":         matSrc,
		"status":                  st,
		"payment_status":          paySt,
		"received_date":           receivedDate.Time,
		"expected_delivery_date":  expectedDate.String,
		"actual_delivery_date":    nil,
		"sub_amount":              subAmt,
		"discount_amount":         discAmt,
		"gst_amount":              gstAmt,
		"net_amount":              netAmt,
		"notes":                   notes,
		"sample_provided":         sampleProvided,
		"sample_description":      sampleDesc,
		"measurement_bill_number": measBillNum,
		"image_url":               imageURL.String,
		"design_image_url":        designImageURL.String,
		"created_by":              createdBy,
		"created_at":              createdAt.Time,
		"updated_at":              updatedAt.Time,
	}
	if actualDate.Valid {
		result["actual_delivery_date"] = actualDate.Time
	}

	// Customer
	var custName, custPhone, custEmail string
	if err := s.db.QueryRow(`SELECT name, phone, COALESCE(email,'') FROM customers WHERE id = $1`, custID).Scan(&custName, &custPhone, &custEmail); err == nil {
		result["customer"] = map[string]interface{}{"id": custID, "name": custName, "phone": custPhone, "email": custEmail}
	}

	// Items
	itemRows, err := s.db.Query(`
		SELECT id, COALESCE(category,''), COALESCE(sub_category,''),
		       COALESCE(pieces, '[]'::jsonb)::text,
		       quantity, unit_price, discount, tax_percent, cgst, sgst, total_price
		FROM job_order_items WHERE job_order_id = $1
	`, id)
	if err == nil {
		defer itemRows.Close()
		var items []map[string]interface{}
		for itemRows.Next() {
			var iid, cat, subCat, piecesStr string
			var qty, up, disc, tp, cgst, sgst, tot float64
			if err := itemRows.Scan(&iid, &cat, &subCat, &piecesStr,
				&qty, &up, &disc, &tp, &cgst, &sgst, &tot); err == nil {
				var pieces []PieceEntry
				if err := json.Unmarshal([]byte(piecesStr), &pieces); err != nil || pieces == nil {
					pieces = []PieceEntry{}
				}
				items = append(items, map[string]interface{}{
					"id":           iid,
					"category":     cat,
					"sub_category": subCat,
					"pieces":       pieces,
					"quantity":     qty,
					"unit_price":   up,
					"discount":     disc,
					"tax_percent":  tp,
					"cgst":         cgst,
					"sgst":         sgst,
					"total_price":  tot,
				})
			}
		}
		if items == nil {
			items = []map[string]interface{}{}
		}
		result["items"] = items
	}

	// Materials
	matRows, err := s.db.Query(`
		SELECT jm.id,
		       COALESCE(jm.raw_material_stock_id::text, '') AS raw_material_stock_id,
		       COALESCE(jm.variant_id::text, '')            AS variant_id,
		       COALESCE(jm.variant_warehouse_id::text, '')  AS variant_warehouse_id,
		       jm.quantity_used,
		       COALESCE(rms.item_name, '')  AS rm_item_name,
		       COALESCE(rms.unit, '')       AS rm_unit,
		       COALESCE(rmw.name, '')       AS rm_warehouse_name,
		       COALESCE(v.variant_code, '') AS variant_code,
		       COALESCE(v.name, '')         AS variant_name,
		       COALESCE(p.name, '')         AS product_name,
		       COALESCE(vw.name, '')        AS variant_warehouse_name
		FROM job_order_materials jm
		LEFT JOIN raw_material_stocks rms ON rms.id = jm.raw_material_stock_id
		LEFT JOIN warehouses rmw ON rmw.id = rms.warehouse_id
		LEFT JOIN variants v   ON v.id = jm.variant_id
		LEFT JOIN products p   ON p.id = v.product_id
		LEFT JOIN warehouses vw ON vw.id = jm.variant_warehouse_id
		WHERE jm.job_order_id = $1
	`, id)
	if err == nil {
		defer matRows.Close()
		var mats []map[string]interface{}
		for matRows.Next() {
			var mid, stockID, variantID, variantWhID string
			var qtyUsed float64
			var rmItemName, rmUnit, rmWhName string
			var variantCode, variantName, productName, variantWhName string
			if err := matRows.Scan(&mid, &stockID, &variantID, &variantWhID, &qtyUsed,
				&rmItemName, &rmUnit, &rmWhName,
				&variantCode, &variantName, &productName, &variantWhName); err == nil {
				mat := map[string]interface{}{
					"id":            mid,
					"quantity_used": qtyUsed,
				}
				if variantID != "" {
					mat["variant_id"] = variantID
					mat["variant_code"] = variantCode
					mat["variant_name"] = variantName
					mat["product_name"] = productName
					mat["item_name"] = productName + " - " + variantName
					mat["warehouse_id"] = variantWhID
					mat["warehouse_name"] = variantWhName
				} else {
					mat["raw_material_stock_id"] = stockID
					mat["item_name"] = rmItemName
					mat["unit"] = rmUnit
					mat["warehouse_name"] = rmWhName
				}
				mats = append(mats, mat)
			}
		}
		if mats == nil {
			mats = []map[string]interface{}{}
		}
		result["materials"] = mats
	}

	// Status history
	shRows, err := s.db.Query(`
		SELECT sh.id, sh.status, sh.notes, sh.updated_by, sh.updated_at,
		       COALESCE(u.name,'') AS updated_by_name
		FROM job_order_status_history sh
		LEFT JOIN users u ON u.id = sh.updated_by
		WHERE sh.job_order_id = $1
		ORDER BY sh.updated_at ASC
	`, id)
	if err == nil {
		defer shRows.Close()
		var history []map[string]interface{}
		for shRows.Next() {
			var sid, sst, snotes, sby, sname string
			var sat sql.NullTime
			if err := shRows.Scan(&sid, &sst, &snotes, &sby, &sat, &sname); err == nil {
				history = append(history, map[string]interface{}{
					"id": sid, "status": sst, "notes": snotes,
					"updated_by": sby, "updated_by_name": sname, "updated_at": sat.Time,
				})
			}
		}
		if history == nil {
			history = []map[string]interface{}{}
		}
		result["status_history"] = history
	}

	// Payments
	payRows, err := s.db.Query(`
		SELECT id, amount, payment_method, reference, paid_at
		FROM job_order_payments WHERE job_order_id = $1
		ORDER BY paid_at ASC
	`, id)
	if err == nil {
		defer payRows.Close()
		var payments []map[string]interface{}
		for payRows.Next() {
			var pid, pm, ref string
			var pamt float64
			var pat sql.NullTime
			if err := payRows.Scan(&pid, &pamt, &pm, &ref, &pat); err == nil {
				payments = append(payments, map[string]interface{}{
					"id": pid, "amount": pamt, "payment_method": pm,
					"reference": ref, "paid_at": pat.Time,
				})
			}
		}
		if payments == nil {
			payments = []map[string]interface{}{}
		}
		result["payments"] = payments

		// Total paid
		var tp float64
		for _, p := range payments {
			tp += p["amount"].(float64)
		}
		result["total_paid"] = tp
		result["balance_due"] = round2(netAmt - tp)
	}

	// Job invoice number
	var invoiceNum sql.NullString
	if err := s.db.QueryRow(`SELECT invoice_number FROM job_invoices WHERE job_order_id = $1`, id).Scan(&invoiceNum); err == nil && invoiceNum.Valid {
		result["invoice_number"] = invoiceNum.String
	}

	return result, nil
}

// ──────────────────────────────────────────
// Cancel
// ──────────────────────────────────────────

func (s *Store) Cancel(id, userID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status, matSrc string
	var whIDNull sql.NullString
	err = tx.QueryRow(`SELECT status, material_source, warehouse_id FROM job_orders WHERE id = $1`, id).Scan(&status, &matSrc, &whIDNull)
	if err != nil {
		return err
	}
	if status == "CANCELLED" {
		return fmt.Errorf("job order already cancelled")
	}
	if status == "DELIVERED" {
		return fmt.Errorf("cannot cancel a delivered job order")
	}

	// Reverse stock if materials were from store
	if matSrc == MaterialSourceStore {
		type matItem struct {
			stockID   sql.NullString
			variantID sql.NullString
			whID      sql.NullString
			qty       float64
		}
		matRows, err := tx.Query(`
			SELECT raw_material_stock_id, variant_id, variant_warehouse_id, quantity_used
			FROM job_order_materials WHERE job_order_id = $1
		`, id)
		if err != nil {
			return err
		}
		var mats []matItem
		for matRows.Next() {
			var m matItem
			if err := matRows.Scan(&m.stockID, &m.variantID, &m.whID, &m.qty); err != nil {
				matRows.Close()
				return err
			}
			mats = append(mats, m)
		}
		matRows.Close()

		for _, m := range mats {
			if m.variantID.Valid && m.variantID.String != "" {
				// Variant-based: restore stock
				_, err = tx.Exec(`
					UPDATE stocks SET quantity = quantity + $1, updated_at = NOW()
					WHERE variant_id = $2 AND warehouse_id = $3
				`, m.qty, m.variantID.String, m.whID.String)
				if err != nil {
					return fmt.Errorf("restore variant stock: %w", err)
				}
				_, err = tx.Exec(`
					INSERT INTO stock_movements
						(variant_id, to_warehouse_id, quantity, movement_type, reference, status)
					VALUES ($1,$2,$3,'JOB_CANCEL',$4,'COMPLETED')
				`, m.variantID.String, m.whID.String, m.qty, "JOB_CANCEL:"+id)
				if err != nil {
					return fmt.Errorf("insert stock_movements for cancel: %w", err)
				}
			} else if m.stockID.Valid && m.stockID.String != "" {
				// Legacy raw material path
				var itemName, rmWhID string
				err = tx.QueryRow(`SELECT item_name, warehouse_id FROM raw_material_stocks WHERE id = $1`, m.stockID.String).Scan(&itemName, &rmWhID)
				if err != nil {
					return fmt.Errorf("raw material stock lookup: %w", err)
				}
				_, err = tx.Exec(`
					UPDATE raw_material_stocks SET quantity = quantity + $1, updated_at = NOW()
					WHERE id = $2
				`, m.qty, m.stockID.String)
				if err != nil {
					return err
				}
				_, err = tx.Exec(`
					INSERT INTO raw_material_movements
						(item_name, warehouse_id, quantity, movement_type, reference)
					VALUES ($1,$2,$3,'IN',$4)
				`, itemName, rmWhID, m.qty, "JOB_CANCEL:"+id)
				if err != nil {
					return err
				}
			}
		}
	}

	// Cancel linked job invoice (if any)
	_, err = tx.Exec(`UPDATE job_invoices SET payment_status = 'CANCELLED' WHERE job_order_id = $1`, id)
	if err != nil {
		return fmt.Errorf("cancel job invoice: %w", err)
	}

	// Delete advance payment records (to be physically refunded to customer)
	_, err = tx.Exec(`DELETE FROM job_order_payments WHERE job_order_id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete payments: %w", err)
	}

	_, err = tx.Exec(`UPDATE job_orders SET status = 'CANCELLED', payment_status = 'UNPAID', updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return err
	}

	// Reverse customer total_purchases
	var custID string
	var netAmt float64
	err = tx.QueryRow(`SELECT customer_id, net_amount FROM job_orders WHERE id = $1`, id).Scan(&custID, &netAmt)
	if err != nil {
		return fmt.Errorf("fetch job order for total_purchases reversal: %w", err)
	}
	_, err = tx.Exec(`
		UPDATE customers
		SET total_purchases = total_purchases - $1, updated_at = NOW()
		WHERE id = $2
	`, netAmt, custID)
	if err != nil {
		return fmt.Errorf("reverse customer total_purchases: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO job_order_status_history (job_order_id, status, notes, updated_by)
		VALUES ($1,'CANCELLED','Cancelled',$2)
	`, id, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ──────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────

func (s *Store) nextJobNumber(tx *sql.Tx) string {
	var max sql.NullString
	tx.QueryRow(`SELECT MAX(job_number) FROM job_orders WHERE job_number LIKE 'JOB%'`).Scan(&max)
	next := 1
	if max.Valid && len(max.String) > 3 {
		fmt.Sscanf(max.String[3:], "%d", &next)
		next++
	}
	return fmt.Sprintf("JOB%05d", next)
}

func (s *Store) nextInvoiceNumber(tx *sql.Tx) string {
	var max sql.NullString
	tx.QueryRow(`SELECT MAX(invoice_number) FROM job_invoices WHERE invoice_number LIKE 'JINV%'`).Scan(&max)
	next := 1
	if max.Valid && len(max.String) > 4 {
		fmt.Sscanf(max.String[4:], "%d", &next)
		next++
	}
	return fmt.Sprintf("JINV%05d", next)
}

"""
Verify sales report CSV against the production database.
Usage: python verify_sales_report.py <path_to_csv>
"""
import sys
import csv
import os
import psycopg2
from collections import defaultdict

CSV_PATH = sys.argv[1] if len(sys.argv) > 1 else r"C:\Users\haria\Downloads\Sales_Report_2026-05-26.csv"

DB_CONFIG = {
    "host":     os.environ["DB_HOST"],
    "port":     int(os.environ["DB_PORT"]),
    "user":     os.environ["DB_USER"],
    "password": os.environ["DB_PASSWORD"],
    "dbname":   os.environ["DB_NAME"],
    "sslmode":  "require",
}

QUERY = """
SELECT
    spm.id::text,
    si.invoice_number,
    TO_CHAR(si.invoice_date AT TIME ZONE 'Asia/Kolkata', 'DD/MM/YYYY') AS date,
    COALESCE(c.name, '')   AS customer_name,
    spm.amount::numeric,
    spm.payment_method,
    COALESCE(b.name, '')   AS location,
    COALESCE(sp.name, '')  AS salesperson_name,
    COALESCE(u.name, '')   AS created_by_name,
    COALESCE(si.channel, '') AS channel
FROM sales_payments spm
JOIN sales_invoices si     ON si.id  = spm.sales_invoice_id
LEFT JOIN customers c      ON c.id   = si.customer_id
LEFT JOIN branches b       ON b.id   = si.branch_id
LEFT JOIN sales_orders so  ON so.id  = si.sales_order_id
LEFT JOIN sales_persons sp ON sp.id  = so.salesperson_id
LEFT JOIN users u          ON u.id::text = si.created_by::text
WHERE b.name = 'DEFAB Thrippunithura'
ORDER BY si.invoice_date DESC, si.invoice_number DESC
"""

def load_csv(path):
    rows = {}
    with open(path, newline="", encoding="utf-8") as f:
        for row in csv.DictReader(f):
            rows[row["id"]] = {
                "invoice_number":  row["invoice_number"],
                "date":            row["date"],
                "customer_name":   row["customer_name"],
                "net_amount":      float(row["net_amount"]),
                "payment_method":  row["payment_method"],
                "location":        row["location"],
                "salesperson_name": row["salesperson_name"],
                "created_by_name": row["created_by_name"],
                "channel":         row["channel"],
            }
    return rows

def load_db(config, query):
    conn = psycopg2.connect(**config)
    cur  = conn.cursor()
    cur.execute(query)
    cols = ["id","invoice_number","date","customer_name","net_amount",
            "payment_method","location","salesperson_name","created_by_name","channel"]
    rows = {}
    for r in cur.fetchall():
        d = dict(zip(cols, r))
        d["net_amount"] = float(d["net_amount"])
        rows[d["id"]] = d
    cur.close()
    conn.close()
    return rows

def main():
    print(f"Loading CSV: {CSV_PATH}")
    csv_rows = load_csv(CSV_PATH)
    print(f"  {len(csv_rows)} rows in CSV")

    print("Connecting to database …")
    db_rows = load_db(DB_CONFIG, QUERY)
    print(f"  {len(db_rows)} rows in DB (DEFAB Thrippunithura, all dates)")

    csv_ids = set(csv_rows)
    db_ids  = set(db_rows)

    only_csv = csv_ids - db_ids
    only_db  = db_ids  - csv_ids
    common   = csv_ids & db_ids

    mismatches = []
    for rid in sorted(common):
        c = csv_rows[rid]
        d = db_rows[rid]
        diffs = {}
        for field in ["invoice_number","date","customer_name","net_amount",
                       "payment_method","location","salesperson_name","created_by_name","channel"]:
            cv, dv = c[field], d[field]
            if field == "net_amount":
                if abs(cv - dv) > 0.01:
                    diffs[field] = (cv, dv)
            else:
                if str(cv).strip() != str(dv).strip():
                    diffs[field] = (cv, dv)
        if diffs:
            mismatches.append((rid, c["invoice_number"], diffs))

    # ── Summary ──────────────────────────────────────────────────
    print("\n" + "="*60)
    print("VERIFICATION SUMMARY")
    print("="*60)
    print(f"  Rows in CSV       : {len(csv_rows)}")
    print(f"  Rows in DB        : {len(db_rows)}")
    print(f"  Matched rows      : {len(common)}")
    print(f"  Mismatched values : {len(mismatches)}")
    print(f"  In CSV only       : {len(only_csv)}")
    print(f"  In DB only (total): {len(only_db)}")

    csv_total = sum(r["net_amount"] for r in csv_rows.values())
    db_total_common = sum(db_rows[rid]["net_amount"] for rid in common)
    csv_total_common = sum(csv_rows[rid]["net_amount"] for rid in common)
    print(f"\n  CSV total (all)   : ₹{csv_total:,.2f}")
    print(f"  CSV total (matched): ₹{csv_total_common:,.2f}")
    print(f"  DB  total (matched): ₹{db_total_common:,.2f}")
    print(f"  Difference        : ₹{abs(csv_total_common - db_total_common):,.2f}")

    if only_csv:
        print(f"\n[ROWS IN CSV BUT NOT IN DB] ({len(only_csv)})")
        for rid in sorted(only_csv):
            r = csv_rows[rid]
            print(f"  {rid}  {r['invoice_number']}  {r['date']}  {r['customer_name']}  ₹{r['net_amount']}")

    if mismatches:
        print(f"\n[FIELD MISMATCHES] ({len(mismatches)})")
        for rid, inv, diffs in mismatches:
            print(f"  {inv} ({rid})")
            for field, (cv, dv) in diffs.items():
                print(f"    {field}: CSV={cv!r}  DB={dv!r}")

    if not only_csv and not mismatches:
        print("\n✓ All CSV rows match the database exactly.")
    elif not only_csv and not mismatches:
        print("\n✓ All matched rows are correct. DB has additional rows not in the CSV filter.")

if __name__ == "__main__":
    main()

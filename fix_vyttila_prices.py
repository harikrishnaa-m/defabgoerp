"""
fix_vyttila_prices.py

Fixes variants.price and variants.cost_price for DEFAB Vyttila that were
corrupted by a parseFloat bug (thousands-separator comma caused "1,775" → 1).

Reads LATEST_STOCK_MAY2026_WITH_GST.xlsx with openpyxl (correct numeric values),
then UPDATE variants SET price, cost_price for every mismatch.

Column layout: col1=CODE, col2=SP_WITH_GST→price, col3=SP_excl, col6=CP→cost_price
Key: (cat_name, item_upper, code) — composite, not just code.

Usage:
  python fix_vyttila_prices.py            # live update
  python fix_vyttila_prices.py --dry-run  # preview only
"""

import sys
import openpyxl
import psycopg2
from psycopg2.extras import execute_values

XLSX_PATH  = r"d:\QMark\defab_erp_backend\internal\migration\Defab Vyttila\LATEST_STOCK_MAY2026_WITH_GST.xlsx"
BRANCH_NAME = "DEFAB Vyttila"
DRY_RUN    = "--dry-run" in sys.argv

DB = dict(
    host="aws-1-ap-southeast-1.pooler.supabase.com",
    port=6543,
    user="postgres.erjlvvznszwlxqdiduhq",
    password="ttH1TANuubY3JoMb",
    dbname="postgres",
    sslmode="require",
)

print(f"Mode: {'DRY RUN' if DRY_RUN else 'LIVE UPDATE'}")
print(f"Excel: {XLSX_PATH}")
print()


def safe_col(row, idx):
    if idx < len(row) and row[idx] is not None:
        return str(row[idx]).strip()
    return ""


def parse_float(s):
    s = str(s).strip().replace(",", "").replace("\u20b9", "").replace(" ", "")
    try:
        return float(s)
    except ValueError:
        return 0.0


# ── 1. Parse Excel ────────────────────────────────────────────────────────────
print("[1] Parsing Excel (col2=SP_WITH_GST, col6=CP)...")
wb = openpyxl.load_workbook(XLSX_PATH, read_only=True, data_only=True)
variant_map = {}  # (cat_name, item_upper, code) → {price, cost_price}

for sheet_name in wb.sheetnames:
    ws = wb[sheet_name]
    all_rows = list(ws.values)

    header_row = -1
    for i, row in enumerate(all_rows[:3]):
        if len(row) > 1 and str(row[1] or "").strip().upper() == "CODE":
            header_row = i
            break
    if header_row < 0:
        continue

    cat_name = sheet_name.strip()
    seen = {}

    for row in all_rows[header_row + 1:]:
        if len(row) < 8:
            continue
        code_str = safe_col(row, 1)
        if not code_str:
            continue
        code = int(parse_float(code_str))
        if code == 0:
            continue

        price_incl = parse_float(safe_col(row, 2))   # SP WITH GST → variants.price
        price_excl = parse_float(safe_col(row, 3))   # SP without GST
        cp         = parse_float(safe_col(row, 6))   # CP → variants.cost_price
        item_desc  = safe_col(row, 11).strip().upper() or cat_name.upper()

        if price_incl <= 0:
            price_incl = price_excl
        cost_price = cp if cp > 0 else (price_excl if price_excl > 0 else price_incl)

        k = (code, item_desc)
        if k not in seen:
            seen[k] = {"price": price_incl, "cost_price": cost_price}

    for (code, item_desc), data in seen.items():
        vkey = (cat_name, item_desc, code)
        if vkey not in variant_map:
            variant_map[vkey] = data

wb.close()
print(f"  Excel variants loaded: {len(variant_map)}")

# ── 2. Query DB ───────────────────────────────────────────────────────────────
print("\n[2] Querying DB...")
conn = psycopg2.connect(**DB)
cur = conn.cursor()
conn.autocommit = False

cur.execute("SELECT id FROM branches WHERE LOWER(name) = LOWER(%s)", (BRANCH_NAME,))
row = cur.fetchone()
if not row:
    print(f"ERROR: Branch '{BRANCH_NAME}' not found"); sys.exit(1)
branch_id = row[0]

cur.execute("SELECT id FROM warehouses WHERE branch_id = %s", (branch_id,))
row = cur.fetchone()
if not row:
    print("ERROR: No warehouse found"); sys.exit(1)
wh_id = row[0]
print(f"  Warehouse: {wh_id}")

cur.execute("""
    SELECT v.id, c.name, p.name, v.variant_code, v.price, v.cost_price
    FROM variants v
    JOIN products p ON p.id = v.product_id
    JOIN categories c ON c.id = p.category_id
    JOIN stocks s ON s.variant_id = v.id AND s.warehouse_id = %s
""", (wh_id,))

db_variants = {}
for vid, cat, item, code, price, cp in cur.fetchall():
    vkey = (cat.strip(), item.strip().upper(), int(code))
    db_variants[vkey] = {"id": vid, "price": float(price or 0), "cost_price": float(cp or 0)}

print(f"  DB variants: {len(db_variants)}")

# ── 3. Find mismatches ────────────────────────────────────────────────────────
print("\n[3] Finding price mismatches...")
to_update = []   # (new_price, new_cp, variant_id)

for vkey, excel_data in variant_map.items():
    if vkey not in db_variants:
        continue
    db_data = db_variants[vkey]
    needs_fix = (
        abs(excel_data["price"] - db_data["price"]) > 0.05 or
        abs(excel_data["cost_price"] - db_data["cost_price"]) > 0.05
    )
    if needs_fix:
        to_update.append((excel_data["price"], excel_data["cost_price"], db_data["id"]))

print(f"  Variants needing fix: {len(to_update)}")
if to_update:
    print("  Sample (first 8 mismatches):")
    shown = 0
    for vkey, excel_data in variant_map.items():
        if vkey not in db_variants:
            continue
        db_data = db_variants[vkey]
        if abs(excel_data["price"] - db_data["price"]) > 0.05:
            cat, item, code = vkey
            print(f"    code={code:<6} {item[:30]:<30} price: {db_data['price']:.2f} → {excel_data['price']:.2f}")
            shown += 1
            if shown >= 8:
                break

if not to_update:
    print("\nAll prices already correct — nothing to fix.")
    cur.close()
    conn.close()
    sys.exit(0)

# ── 4. Apply updates ──────────────────────────────────────────────────────────
if DRY_RUN:
    print(f"\n[DRY RUN] Would update {len(to_update)} variants. Run without --dry-run to apply.")
else:
    print(f"\n[4] Applying {len(to_update)} price updates...")
    execute_values(
        cur,
        """
        UPDATE variants AS v
        SET price = data.new_price,
            cost_price = data.new_cp,
            updated_at = NOW()
        FROM (VALUES %s) AS data(new_price, new_cp, id)
        WHERE v.id = data.id::uuid
        """,
        to_update,
        template="(%s, %s, %s)"
    )
    conn.commit()
    print(f"  Done. {len(to_update)} variants updated.")

cur.close()
conn.close()
print("\nFinished.")

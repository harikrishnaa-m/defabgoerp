"""
verify_vyttila_import.py

Mirrors parseVyttilaXlsx (store.go) exactly, then cross-checks against DB.
Column layout (WITH_GST format):
  col1=CODE, col2=SP(with GST)→variants.price, col3=SP(without GST),
  col4=GST%, col5=GST amount, col6=CP→variants.cost_price, col7=QTY, col11=ITEM DESC
Key: (cat_name, item_desc_upper, code)  — composite, not just code alone.
"""

import sys
import math
from collections import defaultdict
import openpyxl
import psycopg2

XLSX_PATH = (sys.argv[1] if len(sys.argv) > 1
             else r'd:\QMark\defab_erp_backend\internal\migration\Defab Vyttila\LATEST_STOCK_MAY2026_WITH_GST.xlsx')
BRANCH_NAME = "DEFAB Vyttila"

DB = dict(
    host="aws-1-ap-south-1.pooler.supabase.com",
    port=6543,
    user="postgres.zhatpmhkegzyrrmtdzhm",
    password="Defab_staging_123",
    dbname="postgres",
    sslmode="require",
)

print(f"Excel: {XLSX_PATH}")
print(f"Branch: {BRANCH_NAME}")
print()

# ── Helpers ──────────────────────────────────────────────────────────────────
def safe_col(row, idx):
    if idx < len(row) and row[idx] is not None:
        return str(row[idx]).strip()
    return ""

def parse_float(s):
    s = str(s).strip().replace(",", "").replace("₹", "").replace(" ", "")
    try:
        return float(s)
    except ValueError:
        return 0.0

# ── Parse Excel (exact mirror of parseVyttilaXlsx) ───────────────────────────
print("[1] Parsing Excel...")
wb = openpyxl.load_workbook(XLSX_PATH, read_only=True, data_only=True)
variant_map = {}   # (cat_name, item_upper, code) → {price, cost_price, qty}
skipped = []

for sheet_name in wb.sheetnames:
    ws = wb[sheet_name]
    all_rows = list(ws.values)

    header_row = -1
    for i, row in enumerate(all_rows[:3]):
        if len(row) > 1 and str(row[1] or "").strip().upper() == "CODE":
            header_row = i
            break
    if header_row < 0:
        skipped.append(sheet_name)
        continue

    cat_name = sheet_name.strip()
    seen = {}   # (code, item) → entry dict

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
        qty        = parse_float(safe_col(row, 7))
        item_desc  = safe_col(row, 11).strip().upper()
        if not item_desc:
            item_desc = cat_name.upper()

        if price_incl <= 0:
            price_incl = price_excl
        cost_price = cp if cp > 0 else (price_excl if price_excl > 0 else price_incl)

        k = (code, item_desc)
        if k in seen:
            seen[k]["qty"] += qty
        else:
            seen[k] = {"price": price_incl, "cost_price": cost_price, "qty": qty}

    sheet_total = 0
    for (code, item_desc), data in seen.items():
        vkey = (cat_name, item_desc, code)
        if vkey in variant_map:
            variant_map[vkey]["qty"] += data["qty"]
        else:
            variant_map[vkey] = {"price": data["price"], "cost_price": data["cost_price"], "qty": round(data["qty"]*100)/100}
        sheet_total += 1

    print(f"  Sheet {sheet_name!r:<35} → {sheet_total} unique codes")

wb.close()
if skipped:
    print(f"  Skipped (no CODE header): {skipped}")
print(f"  Total Excel variants: {len(variant_map)}")

# Per-category Excel summary
cat_ex = defaultdict(lambda: {"v": 0, "qty": 0.0})
for (cat, item, code), d in variant_map.items():
    cat_ex[cat]["v"] += 1
    cat_ex[cat]["qty"] += d["qty"]
print("\n  Per-category (Excel):")
for cat in sorted(cat_ex):
    print(f"    {cat:<40} variants={cat_ex[cat]['v']:>5}  total_qty={cat_ex[cat]['qty']:>10.2f}")

# ── DB Query ─────────────────────────────────────────────────────────────────
print("\n[2] Querying DB...")
conn = psycopg2.connect(**DB)
cur = conn.cursor()

cur.execute("SELECT id FROM branches WHERE LOWER(name) = LOWER(%s)", (BRANCH_NAME,))
row = cur.fetchone()
if not row:
    print(f"ERROR: Branch '{BRANCH_NAME}' not found"); sys.exit(1)
branch_id = row[0]
print(f"  Branch ID : {branch_id}")

cur.execute("SELECT id FROM warehouses WHERE branch_id = %s", (branch_id,))
row = cur.fetchone()
if not row:
    print(f"ERROR: No warehouse found"); sys.exit(1)
wh_id = row[0]
print(f"  Warehouse : {wh_id}")

cur.execute("""
    SELECT c.name, p.name, v.variant_code, v.price, v.cost_price, s.quantity
    FROM variants v
    JOIN products p ON p.id = v.product_id
    JOIN categories c ON c.id = p.category_id
    JOIN stocks s ON s.variant_id = v.id AND s.warehouse_id = %s
    ORDER BY c.name, p.name, v.variant_code
""", (wh_id,))

db_map = {}
for cat_name, item_name, code, price, cost_price, qty in cur.fetchall():
    vkey = (cat_name.strip(), item_name.strip().upper(), int(code))
    db_map[vkey] = {"price": float(price or 0), "cost_price": float(cost_price or 0), "qty": float(qty or 0)}

cur.close()
conn.close()
print(f"  Total DB variants: {len(db_map)}")

cat_db = defaultdict(lambda: {"v": 0, "qty": 0.0})
for (cat, item, code), d in db_map.items():
    cat_db[cat]["v"] += 1
    cat_db[cat]["qty"] += d["qty"]
print("\n  Per-category (DB):")
for cat in sorted(cat_db):
    print(f"    {cat:<40} variants={cat_db[cat]['v']:>5}  total_qty={cat_db[cat]['qty']:>10.2f}")

# ── Compare ───────────────────────────────────────────────────────────────────
print("\n[3] Comparing...")
excel_keys = set(variant_map.keys())
db_keys    = set(db_map.keys())

missing_in_db  = sorted(excel_keys - db_keys)
extra_in_db    = sorted(db_keys - excel_keys)
mismatches     = []

for k in sorted(excel_keys & db_keys):
    e, d = variant_map[k], db_map[k]
    diffs = []
    if abs(e["price"] - d["price"]) > 0.05:
        diffs.append(f"price: excel={e['price']:.2f}  db={d['price']:.2f}")
    if abs(e["cost_price"] - d["cost_price"]) > 0.05:
        diffs.append(f"cost_price: excel={e['cost_price']:.2f}  db={d['cost_price']:.2f}")
    excel_qty = round(e["qty"]*100)/100
    db_qty    = round(d["qty"]*100)/100
    if abs(excel_qty - db_qty) > 0.01:
        diffs.append(f"qty: excel={excel_qty:.2f}  db={db_qty:.2f}")
    if diffs:
        mismatches.append((k, diffs))

# ── Report ────────────────────────────────────────────────────────────────────
print()
print("=" * 70)
print("RESULTS")
print("=" * 70)

if missing_in_db:
    print(f"\n[MISSING IN DB] {len(missing_in_db)} variants in Excel but not in DB:")
    for (cat, item, code) in missing_in_db[:30]:
        e = variant_map[(cat, item, code)]
        print(f"  code={code:<6}  cat={cat!r:<30}  item={item!r:<40}  price={e['price']:.2f}  qty={e['qty']:.2f}")
    if len(missing_in_db) > 30:
        print(f"  ... and {len(missing_in_db)-30} more")

if extra_in_db:
    print(f"\n[EXTRA IN DB] {len(extra_in_db)} variants in DB but not in Excel:")
    for (cat, item, code) in extra_in_db[:30]:
        d = db_map[(cat, item, code)]
        print(f"  code={code:<6}  cat={cat!r:<30}  item={item!r:<40}  price={d['price']:.2f}  qty={d['qty']:.2f}")
    if len(extra_in_db) > 30:
        print(f"  ... and {len(extra_in_db)-30} more")

if mismatches:
    print(f"\n[VALUE MISMATCHES] {len(mismatches)} variants with differing values:")
    for (cat, item, code), diffs in mismatches[:30]:
        print(f"  code={code:<6}  cat={cat!r:<30}  item={item!r}")
        for diff in diffs:
            print(f"      ↳ {diff}")
    if len(mismatches) > 30:
        print(f"  ... and {len(mismatches)-30} more")

total_issues = len(missing_in_db) + len(extra_in_db) + len(mismatches)
print()
if total_issues == 0:
    print("ALL OK — DB perfectly matches Excel.")
else:
    print(f"Issues found: {len(missing_in_db)} missing  |  {len(extra_in_db)} extra  |  {len(mismatches)} value mismatches")

print(f"\nExcel: {len(variant_map)} variants | DB: {len(db_map)} variants | Issues: {total_issues}")

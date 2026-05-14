"""
fix_vyttila_all.py

Comprehensive fix: finds ALL Vyttila variants (including duplicates with the same key)
and updates EVERY one whose price doesn't match Excel.
The previous fix missed duplicates because it stored only one UUID per key.
"""

import sys
import openpyxl
import psycopg2
from psycopg2.extras import execute_values
from collections import defaultdict

XLSX_PATH   = r"d:\QMark\defab_erp_backend\internal\migration\Defab Vyttila\LATEST_STOCK_MAY2026_WITH_GST.xlsx"
BRANCH_NAME = "DEFAB Vyttila"
DRY_RUN     = "--dry-run" in sys.argv

DB = dict(
    host="aws-1-ap-south-1.pooler.supabase.com",
    port=6543,
    user="postgres.zhatpmhkegzyrrmtdzhm",
    password="Defab_staging_123",
    dbname="postgres",
    sslmode="require",
)

print(f"Mode : {'DRY RUN' if DRY_RUN else 'LIVE UPDATE'}")
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

# ── 1. Parse Excel ─────────────────────────────────────────────────────────
print("[1] Parsing Excel...")
wb = openpyxl.load_workbook(XLSX_PATH, read_only=True, data_only=True)
excel_map = {}   # (cat_name, item_upper, code) → {price, cost_price}

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
        code = int(parse_float(safe_col(row, 1)))
        if code == 0:
            continue
        price_incl = parse_float(safe_col(row, 2))
        price_excl = parse_float(safe_col(row, 3))
        cp         = parse_float(safe_col(row, 6))
        item_desc  = safe_col(row, 11).strip().upper() or cat_name.upper()
        if price_incl <= 0:
            price_incl = price_excl
        cost_price = cp if cp > 0 else (price_excl if price_excl > 0 else price_incl)
        k = (code, item_desc)
        if k not in seen:
            seen[k] = {"price": price_incl, "cost_price": cost_price}

    for (code, item_desc), data in seen.items():
        vkey = (cat_name, item_desc, code)
        if vkey not in excel_map:
            excel_map[vkey] = data

wb.close()
print(f"  Excel variants: {len(excel_map)}")

# ── 2. Query DB — ALL rows, no deduplication ────────────────────────────────
print("\n[2] Querying DB (all rows, no dedup)...")
conn = psycopg2.connect(**DB)
cur  = conn.cursor()
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

# Store ALL rows — multiple UUIDs per key if duplicates exist
key_to_rows = defaultdict(list)
for vid, cat, item, code, price, cp in cur.fetchall():
    vkey = (cat.strip(), item.strip().upper(), int(code))
    key_to_rows[vkey].append({
        "id":         vid,
        "price":      float(price or 0),
        "cost_price": float(cp or 0),
    })

total_db_rows = sum(len(v) for v in key_to_rows.values())
dup_keys = [(k, v) for k, v in key_to_rows.items() if len(v) > 1]
print(f"  Total DB variant rows : {total_db_rows}")
print(f"  Unique (cat,item,code): {len(key_to_rows)}")
print(f"  Keys with duplicates  : {len(dup_keys)}")
if dup_keys:
    print("  Sample duplicates:")
    for k, entries in dup_keys[:5]:
        cat, item, code = k
        print(f"    cat={cat!r} item={item!r} code={code}")
        for e in entries:
            print(f"      id={e['id']}  price={e['price']}  cp={e['cost_price']}")

# ── 3. Build update list — check EVERY variant UUID ────────────────────────
print("\n[3] Finding mismatches (checking every UUID)...")
to_update = []   # (new_price, new_cp, variant_id_str)

for vkey, db_rows in key_to_rows.items():
    excel_data = excel_map.get(vkey)
    if excel_data is None:
        continue   # not in our Excel file — skip
    for db_entry in db_rows:
        needs_fix = (
            abs(excel_data["price"]      - db_entry["price"])      > 0.05 or
            abs(excel_data["cost_price"] - db_entry["cost_price"]) > 0.05
        )
        if needs_fix:
            to_update.append((
                excel_data["price"],
                excel_data["cost_price"],
                str(db_entry["id"]),
            ))

print(f"  Variants needing price fix: {len(to_update)}")
if to_update:
    print("  Sample (first 10):")
    shown = 0
    for vkey, db_rows in key_to_rows.items():
        excel_data = excel_map.get(vkey)
        if not excel_data:
            continue
        for db_entry in db_rows:
            if abs(excel_data["price"] - db_entry["price"]) > 0.05:
                cat, item, code = vkey
                print(f"    code={code:<6} {item[:35]:<35} "
                      f"price: {db_entry['price']:.2f} → {excel_data['price']:.2f}")
                shown += 1
                if shown >= 10:
                    break
        if shown >= 10:
            break

if not to_update:
    print("\nAll prices already correct — nothing to fix.")
    cur.close(); conn.close(); sys.exit(0)

# ── 4. Apply updates ─────────────────────────────────────────────────────────
if DRY_RUN:
    print(f"\n[DRY RUN] Would fix {len(to_update)} variants. Run without --dry-run to apply.")
else:
    print(f"\n[4] Applying {len(to_update)} updates...")
    execute_values(
        cur,
        """
        UPDATE variants AS v
        SET price      = data.new_price::numeric,
            cost_price = data.new_cp::numeric,
            updated_at = NOW()
        FROM (VALUES %s) AS data(new_price, new_cp, id)
        WHERE v.id = data.id::uuid
        """,
        to_update,
        template="(%s, %s, %s)",
    )
    conn.commit()
    print(f"  Done — {len(to_update)} variants updated.")

cur.close()
conn.close()
print("\nFinished.")

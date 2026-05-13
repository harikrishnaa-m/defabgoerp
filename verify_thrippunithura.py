"""
Verify that the Defab Thrippunithura DB data matches the Excel files.
Checks per variant-code (within each category+product):
  - price (DB) == MRP from Excel (GST-inclusive)
  - cost_price (DB) == MRP Excl. GST from Excel
  - stock quantity (DB, Thrippunithura warehouse) == sum of qty from Excel
"""
import os, re
import psycopg2
import openpyxl
from collections import defaultdict

# ── DB connection ─────────────────────────────────────────────
conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com',
    port=6543,
    user='postgres.erjlvvznszwlxqdiduhq',
    password='ttH1TANuubY3JoMb',
    dbname='postgres',
    sslmode='require'
)
cur = conn.cursor()

# ── Find Thrippunithura warehouse ─────────────────────────────
cur.execute("""
    SELECT w.id, w.name
    FROM warehouses w
    JOIN branches b ON b.id = w.branch_id
    WHERE LOWER(b.name) LIKE '%thrippunithura%'
    ORDER BY w.created_at
    LIMIT 1
""")
row = cur.fetchone()
if not row:
    print("ERROR: Could not find Thrippunithura warehouse in DB")
    exit(1)
warehouse_id, warehouse_name = row
print(f"Warehouse: {warehouse_name}  ({warehouse_id})\n")

# ── Load DB data ───────────────────────────────────────────────
# key: (lower_cat_name, lower_item_name, variant_code)
# value: { price, cost_price, qty }
cur.execute("""
    SELECT
        LOWER(c.name)        AS cat,
        LOWER(p.name)        AS product,
        v.variant_code       AS code,
        v.price              AS mrp,
        v.cost_price         AS cost_price,
        COALESCE(st.quantity, 0) AS qty
    FROM variants v
    JOIN products p ON p.id = v.product_id
    JOIN categories c ON c.id = p.category_id
    LEFT JOIN stocks st ON st.variant_id = v.id AND st.warehouse_id = %s
    WHERE LOWER(c.name) IN (
        'saree','churidhar and salwaar','dybl mtrl','dupatta','kurthis','running mtl'
    )
""", (str(warehouse_id),))

db = {}
for cat, product, code, mrp, cost_price, qty in cur.fetchall():
    key = (cat, product, code)
    db[key] = {'mrp': float(mrp or 0), 'cost_price': float(cost_price or 0), 'qty': float(qty or 0)}

print(f"DB variants loaded: {len(db)}\n")

# ── Parse Excel files ──────────────────────────────────────────
folder = r"internal\migration\Defab Thrippunithura"

def detect_cols(rows):
    for i, row in enumerate(rows[:3]):
        vals = [str(c).strip().upper() if c else '' for c in row]
        col_item = col_code = col_qty = col_mrp = col_mrp_excl = -1
        for j, v in enumerate(vals):
            if col_item < 0 and ('ITEM' in v or 'ITEAM' in v): col_item = j
            if col_code < 0 and v == 'CODE': col_code = j
            if col_qty < 0 and v == 'QTY': col_qty = j
            if col_mrp_excl < 0 and 'MRP' in v and 'EXCL' in v: col_mrp_excl = j
            elif col_mrp < 0 and 'MRP' in v and 'EXCL' not in v: col_mrp = j
        if col_code >= 0:
            if col_item < 0:
                col_item = 2 if col_code == 1 else 1
            return i, col_item, col_code, col_mrp, col_mrp_excl, col_qty
    return None, -1, -1, -1, -1, -1

# Excel data keyed same way: (lower_cat, lower_item, code) -> {mrp, cost_price, qty}
# For variants with same key across sheets/files, sum qty and take last MRP
excel = defaultdict(lambda: {'mrp': 0.0, 'cost_price': 0.0, 'qty': 0.0})

for fname in sorted(os.listdir(folder)):
    if not fname.endswith('.xlsx'): continue
    cat = re.sub(r'\.xlsx$', '', fname, flags=re.IGNORECASE).strip().lower()
    wb = openpyxl.load_workbook(os.path.join(folder, fname), data_only=True)
    for sh in wb.worksheets:
        rows = list(sh.iter_rows(values_only=True))
        hdr, ci, cc, cm, cme, cq = detect_cols(rows)
        if hdr is None: continue
        for row in rows[hdr+1:]:
            try:
                code_val = row[cc] if cc < len(row) else None
                if not code_val or str(code_val).strip() in ('', 'None'): continue
                code = int(float(str(code_val)))
                if code <= 0: continue
                item = str(row[ci]).strip() if ci < len(row) and row[ci] else ''
                if not item: item = cat
                item = item.lower()

                def fval(col):
                    if col < 0 or col >= len(row) or row[col] is None: return 0.0
                    try: return float(str(row[col]).strip() or 0)
                    except: return 0.0

                mrp      = fval(cm)   # GST-inclusive
                cost_p   = fval(cme)  # MRP excl. GST
                qty      = fval(cq)

                # If no GST-inclusive column, fall back to excl column
                if mrp <= 0 and cost_p > 0:
                    mrp = cost_p
                    cost_p = 0.0

                key = (cat, item, code)
                excel[key]['qty'] += qty
                if mrp > 0:
                    excel[key]['mrp'] = mrp
                if cost_p > 0:
                    excel[key]['cost_price'] = cost_p
            except: pass

print(f"Excel variants parsed: {len(excel)}\n")

# ── Compare ────────────────────────────────────────────────────
price_mismatches  = []
cost_mismatches   = []
qty_mismatches    = []
missing_in_db     = []
extra_in_db       = []

for key, ev in excel.items():
    if key not in db:
        missing_in_db.append(key)
        continue
    dv = db[key]

    # price check (allow ±0.01 rounding)
    if ev['mrp'] > 0 and abs(dv['mrp'] - ev['mrp']) > 0.01:
        price_mismatches.append({
            'key': key, 'excel_mrp': ev['mrp'], 'db_price': dv['mrp'],
            'diff': round(dv['mrp'] - ev['mrp'], 2)
        })

    # cost_price check
    if ev['cost_price'] > 0 and abs(dv['cost_price'] - ev['cost_price']) > 0.01:
        cost_mismatches.append({
            'key': key, 'excel_cost': ev['cost_price'], 'db_cost': dv['cost_price'],
            'diff': round(dv['cost_price'] - ev['cost_price'], 2)
        })

    # qty check (allow ±0.01)
    if abs(dv['qty'] - ev['qty']) > 0.01:
        qty_mismatches.append({
            'key': key, 'excel_qty': round(ev['qty'],2), 'db_qty': round(dv['qty'],2),
            'diff': round(dv['qty'] - ev['qty'], 2)
        })

for key in db:
    if key not in excel:
        extra_in_db.append(key)

# ── Print summary ──────────────────────────────────────────────
def show(title, items, limit=10):
    print(f"{'='*60}")
    print(f"{title}: {len(items)}")
    for x in items[:limit]:
        print(f"  {x}")
    if len(items) > limit:
        print(f"  ... and {len(items)-limit} more")
    print()

print("\n" + "="*60)
print("VERIFICATION SUMMARY")
print("="*60)
print(f"  Excel variants     : {len(excel)}")
print(f"  DB variants        : {len(db)}")
print(f"  Missing in DB      : {len(missing_in_db)}")
print(f"  Extra in DB (not in Excel): {len(extra_in_db)}")
print(f"  Price mismatches   : {len(price_mismatches)}")
print(f"  Cost price mismatch: {len(cost_mismatches)}")
print(f"  Qty mismatches     : {len(qty_mismatches)}")
print()

if price_mismatches:
    show("PRICE MISMATCHES (cat, item, code) | excel_mrp | db_price | diff", price_mismatches)
if cost_mismatches:
    show("COST PRICE MISMATCHES", cost_mismatches)
if qty_mismatches:
    show("QTY MISMATCHES (first 20)", qty_mismatches, limit=20)
if missing_in_db:
    show("MISSING IN DB (first 10)", missing_in_db)
if extra_in_db:
    show("EXTRA IN DB / NOT IN EXCEL (first 10)", extra_in_db)

if not any([price_mismatches, cost_mismatches, qty_mismatches, missing_in_db]):
    print("✓ All matched variants have correct price, cost_price, and qty!")

cur.close()
conn.close()

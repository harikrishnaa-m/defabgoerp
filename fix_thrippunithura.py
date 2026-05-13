"""
Fix Thrippunithura DB:
1. Correct 2 wrong prices (dupatta/orgnza/995 and saree/prntd mx sr/5520)
2. Set RUNNING MTL lace quantities to Excel values (undo double-import)
"""
import os, re
import psycopg2
import openpyxl
from collections import defaultdict

conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com',
    port=6543,
    user='postgres.erjlvvznszwlxqdiduhq',
    password='ttH1TANuubY3JoMb',
    dbname='postgres',
    sslmode='require'
)
conn.autocommit = False
cur = conn.cursor()

# ── Warehouse ──────────────────────────────────────────────────
cur.execute("""
    SELECT w.id FROM warehouses w
    JOIN branches b ON b.id = w.branch_id
    WHERE LOWER(b.name) LIKE '%thrippunithura%'
    ORDER BY w.created_at LIMIT 1
""")
warehouse_id = cur.fetchone()[0]
print(f"Warehouse: {warehouse_id}\n")

# ── 1. Fix 2 price errors ──────────────────────────────────────
price_fixes = [
    # (variant_code, lower_product_name, lower_cat_name, correct_price, correct_cost_price)
    (995,  'orgnza',      'dupatta', 305.00, 290.48),
    (5520, 'prntd mx sr', 'saree',  2475.00, 2357.14),
]

for code, prod, cat, price, cost in price_fixes:
    cur.execute("""
        UPDATE variants v
        SET price = %s, cost_price = %s, updated_at = NOW()
        FROM products p JOIN categories c ON c.id = p.category_id
        WHERE v.product_id = p.id
          AND v.variant_code = %s
          AND LOWER(p.name) = %s
          AND LOWER(c.name) = %s
    """, (price, cost, code, prod, cat))
    print(f"Price fix  [{cat}/{prod}/code={code}]: {cur.rowcount} row(s) updated  →  price={price}, cost_price={cost}")

# ── 2. Parse Excel for RUNNING MTL to get correct quantities ──
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

# Collect correct Excel quantities for RUNNING MTL only
# key: (lower_item, code) → correct_qty
running_qty = defaultdict(float)

for fname in sorted(os.listdir(folder)):
    if not fname.endswith('.xlsx'): continue
    cat = re.sub(r'\.xlsx$', '', fname, flags=re.IGNORECASE).strip().lower()
    if cat != 'running mtl': continue

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
                item = str(row[ci]).strip().lower() if ci < len(row) and row[ci] else ''
                qty_val = row[cq] if cq >= 0 and cq < len(row) else None
                qty = float(str(qty_val).strip() or 0) if qty_val else 0.0
                running_qty[(item, code)] += qty
            except: pass

print(f"\nRUNNING MTL Excel keys parsed: {len(running_qty)}")

# ── 3. Bulk-correct RUNNING MTL stock quantities ──────────────
# Build a temp table of (item_name, variant_code, correct_qty) and do one UPDATE
if running_qty:
    cur.execute("""
        CREATE TEMP TABLE _qty_fix (
            item_name TEXT,
            code      INT,
            correct_qty NUMERIC
        )
    """)

    items = list(running_qty.items())
    chunk = 500
    for i in range(0, len(items), chunk):
        batch = items[i:i+chunk]
        vals = []
        args = []
        for j, ((item, code), qty) in enumerate(batch):
            base = j * 3
            vals.append(f"(${base+1}, ${base+2}, ${base+3})")
            args.extend([item, code, qty])
        cur.execute(f"INSERT INTO _qty_fix VALUES {', '.join(vals)}", args)

    cur.execute("""
        UPDATE stocks st
        SET quantity = qf.correct_qty, updated_at = NOW()
        FROM _qty_fix qf
        JOIN variants v  ON v.variant_code = qf.code
        JOIN products p  ON p.id = v.product_id
        JOIN categories c ON c.id = p.category_id
        WHERE st.variant_id  = v.id
          AND st.warehouse_id = %s
          AND LOWER(c.name)   = 'running mtl'
          AND LOWER(p.name)   = qf.item_name
          AND ABS(st.quantity - qf.correct_qty) > 0.01
    """, (str(warehouse_id),))
    fixed_qty = cur.rowcount
    print(f"Qty rows corrected: {fixed_qty}")
else:
    print("No RUNNING MTL data parsed — skipping qty fix")

conn.commit()
print("\nAll corrections committed.")
cur.close()
conn.close()

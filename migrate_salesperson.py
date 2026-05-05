"""
Migration script:
1. Parse SalesRepot.xls -> POS number: salesperson name mapping
2. Upsert salespersons into sales_persons (create if not exists by name, case-insensitive, for the TPTRA branch)
3. Update sales_orders.salesperson_id for all matched POS invoices
4. Delete POS1xxx invoices and their related records (invoice items, order items, orders)
"""

import psycopg2
import uuid
import pandas as pd

DB = dict(
    host='aws-1-ap-southeast-1.pooler.supabase.com',
    port=5432,
    user='postgres.erjlvvznszwlxqdiduhq',
    password='ttH1TANuubY3JoMb',
    dbname='postgres',
    sslmode='require'
)

TPTRA_BRANCH_ID = 'f03469df-dada-4c10-a276-0a4d17416680'
XLS_PATH = 'internal/migration/SalesMigration/SalesRepot.xls'

# ============================================================
# STEP 1: Parse SalesRepot.xls -> {pos_number: salesperson_name}
# ============================================================
print("=" * 60)
print("STEP 1: Parsing SalesRepot.xls")
tables = pd.read_html(XLS_PATH, encoding='utf-8')

pos_to_sp = {}
for t in tables:
    if t.shape == (1, 7) and isinstance(t.iloc[0, 1], str) and '/' in str(t.iloc[0, 1]):
        ref = str(t.iloc[0, 1])
        parts = ref.split('/')
        if len(parts) >= 2:
            pos_num = parts[1].strip()  # e.g. POS4811
            salesperson = str(t.iloc[0, 5]).strip()
            if pos_num.startswith('POS'):
                pos_to_sp[pos_num] = salesperson

print(f"  Parsed {len(pos_to_sp)} POS -> salesperson mappings")
unique_sps = sorted(set(pos_to_sp.values()))
print(f"  Unique salespersons ({len(unique_sps)}): {unique_sps}")

# ============================================================
# STEP 2: Connect and upsert salespersons
# ============================================================
print("\n" + "=" * 60)
print("STEP 2: Upsert salespersons")
conn = psycopg2.connect(**DB)
cur = conn.cursor()

# Existing salespersons in this branch (case-insensitive lookup)
cur.execute("SELECT id, name FROM sales_persons WHERE branch_id = %s", (TPTRA_BRANCH_ID,))
existing = {name.lower().strip(): id for id, name in cur.fetchall()}
print(f"  Existing salespersons in TPTRA branch: {list(existing.keys())}")

# Find next employee code number
cur.execute("SELECT employee_code FROM sales_persons WHERE employee_code ~ '^SP[0-9]+$' ORDER BY CAST(SUBSTRING(employee_code FROM 3) AS INTEGER) DESC LIMIT 1")
row = cur.fetchone()
next_sp_num = int(row[0][2:]) + 1 if row else 10

sp_name_to_id = {}
for name in unique_sps:
    name_key = name.lower().strip()
    if name_key in existing:
        sp_name_to_id[name] = existing[name_key]
        print(f"  EXISTING: '{name}' -> {existing[name_key]}")
    else:
        new_id = str(uuid.uuid4())
        emp_code = f"SP{next_sp_num:03d}"
        next_sp_num += 1
        safe_email = name.lower().replace(' ', '.') + "@tptra.migration"
        cur.execute(
            """INSERT INTO sales_persons (id, user_id, branch_id, name, employee_code, phone, email, is_active, created_at)
               VALUES (%s, NULL, %s, %s, %s, %s, %s, true, NOW())""",
            (new_id, TPTRA_BRANCH_ID, name, emp_code, '0000000000', safe_email)
        )
        sp_name_to_id[name] = new_id
        print(f"  CREATED: '{name}' -> {new_id} ({emp_code})")

# ============================================================
# STEP 3: Update sales_orders.salesperson_id
# ============================================================
print("\n" + "=" * 60)
print("STEP 3: Updating sales_orders.salesperson_id")

updated = 0
not_in_db = []
for pos_num, sp_name in pos_to_sp.items():
    sp_id = sp_name_to_id.get(sp_name)
    if not sp_id:
        not_in_db.append(pos_num)
        continue
    cur.execute(
        """UPDATE sales_orders so
           SET salesperson_id = %s
           FROM sales_invoices si
           WHERE so.id = si.sales_order_id AND si.invoice_number = %s""",
        (sp_id, pos_num)
    )
    if cur.rowcount > 0:
        updated += 1
    else:
        not_in_db.append(pos_num)

print(f"  Updated salesperson_id on {updated} sales_orders")
if not_in_db:
    print(f"  Invoice numbers not found in DB ({len(not_in_db)}): {not_in_db[:20]}")

# ============================================================
# STEP 4: Delete POS1xxx invoices and related records
# ============================================================
print("\n" + "=" * 60)
print("STEP 4: Deleting POS1xxx invoices (non-TPTRA branch)")

cur.execute("SELECT id, sales_order_id FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
rows = cur.fetchall()
inv_ids = [r[0] for r in rows]
so_ids = [r[1] for r in rows if r[1] is not None]

print(f"  Found {len(inv_ids)} POS1xxx invoices, {len(so_ids)} linked sales_orders")

if inv_ids:
    cur.execute("DELETE FROM sales_invoice_items WHERE sales_invoice_id = ANY(%s::uuid[])", (inv_ids,))
    print(f"  Deleted {cur.rowcount} sales_invoice_items")

    cur.execute("DELETE FROM sales_invoices WHERE id = ANY(%s::uuid[])", (inv_ids,))
    print(f"  Deleted {cur.rowcount} sales_invoices")

if so_ids:
    cur.execute("DELETE FROM sales_order_items WHERE sales_order_id = ANY(%s::uuid[])", (so_ids,))
    print(f"  Deleted {cur.rowcount} sales_order_items")

    cur.execute("DELETE FROM sales_orders WHERE id = ANY(%s::uuid[])", (so_ids,))
    print(f"  Deleted {cur.rowcount} sales_orders")

# ============================================================
# Commit all
# ============================================================
conn.commit()
print("\n" + "=" * 60)
print("ALL CHANGES COMMITTED SUCCESSFULLY.")
conn.close()

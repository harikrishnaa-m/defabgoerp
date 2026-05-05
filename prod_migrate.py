"""
Production migration:
1. Map POS invoices -> salesperson using existing prod salespersons (case-insensitive)
2. Create 'sajitha ps' (not in prod yet)
3. Update sales_orders.salesperson_id for 373 POS invoices
4. Delete 76 POS1xxx invoices + related records
"""
import psycopg2
import uuid
import pandas as pd
import bcrypt

PROD_DB = dict(
    host='aws-1-ap-south-1.pooler.supabase.com', port=5432,
    user='postgres.zhatpmhkegzyrrmtdzhm', password='Defab_staging_123',
    dbname='postgres', sslmode='require'
)
PROD_BRANCH_ID = '40b2f6f2-f6b4-4498-9428-8925f14f577f'
XLS_PATH = 'internal/migration/SalesMigration/SalesRepot.xls'
PLACEHOLDER_PASSWORD = 'Tptra@2024'

# ============================================================
# STEP 1: Parse SalesRepot.xls -> {pos_number: salesperson_name}
# ============================================================
print("=" * 60)
print("STEP 1: Parsing SalesRepot.xls")
tables = pd.read_html(XLS_PATH, encoding='utf-8')

pos_to_sp = {}
for t in tables:
    if t.shape == (1, 7) and isinstance(t.iloc[0, 1], str) and '/' in str(t.iloc[0, 1]):
        parts = str(t.iloc[0, 1]).split('/')
        if len(parts) >= 2:
            pos_num = parts[1].strip()
            salesperson = str(t.iloc[0, 5]).strip()
            if pos_num.startswith('POS'):
                pos_to_sp[pos_num] = salesperson

print(f"  Parsed {len(pos_to_sp)} POS -> salesperson mappings")
unique_sps = sorted(set(pos_to_sp.values()))
print(f"  Unique salespersons in XLS: {unique_sps}")

# ============================================================
# STEP 2: Match XLS names to prod salespersons
# ============================================================
print("\n" + "=" * 60)
print("STEP 2: Matching to production salespersons")

conn = psycopg2.connect(**PROD_DB)
cur = conn.cursor()

cur.execute("SELECT id, name, employee_code FROM sales_persons WHERE branch_id = %s", (PROD_BRANCH_ID,))
existing = cur.fetchall()
# Build case-insensitive + normalize lookup (strip spaces, lower)
existing_lookup = {name.lower().strip(): (id, emp_code) for id, name, emp_code in existing}
print(f"  Existing: {[(e[1], e[2]) for e in existing]}")

# Find next employee code number
cur.execute("SELECT employee_code FROM sales_persons WHERE employee_code ~ '^SP[0-9]+$' ORDER BY CAST(SUBSTRING(employee_code FROM 3) AS INTEGER) DESC LIMIT 1")
row = cur.fetchone()
next_sp_num = int(row[0][2:]) + 1 if row else 9

password_hash = bcrypt.hashpw(PLACEHOLDER_PASSWORD.encode(), bcrypt.gensalt(rounds=12)).decode()

sp_name_to_id = {}
for xls_name in unique_sps:
    key = xls_name.lower().strip()
    if key in existing_lookup:
        sp_id, emp_code = existing_lookup[key]
        sp_name_to_id[xls_name] = sp_id
        print(f"  MATCHED: '{xls_name}' -> {emp_code} (exact)")
    else:
        # Try fuzzy: compare first 5 chars lower (handles MAHESWARI vs Maheswary)
        matched = None
        for db_key, (sp_id, emp_code) in existing_lookup.items():
            if key[:6] == db_key[:6]:
                matched = (sp_id, emp_code, db_key)
                break
        if matched:
            sp_id, emp_code, db_key = matched
            sp_name_to_id[xls_name] = sp_id
            print(f"  FUZZY MATCH: '{xls_name}' -> '{db_key}' ({emp_code})")
        else:
            # Create new salesperson + user
            new_sp_id = str(uuid.uuid4())
            new_user_id = str(uuid.uuid4())
            emp_code = f"SP{next_sp_num:03d}"
            next_sp_num += 1
            email = f"{emp_code.lower()}@tptra.migration"

            cur.execute(
                """INSERT INTO users (id, name, email, password_hash, branch_id, is_active, employee_code, created_at)
                   VALUES (%s, %s, %s, %s, %s, true, %s, NOW())""",
                (new_user_id, xls_name, email, password_hash, PROD_BRANCH_ID, emp_code)
            )
            cur.execute(
                """INSERT INTO sales_persons (id, user_id, branch_id, name, employee_code, phone, email, is_active, created_at)
                   VALUES (%s, %s, %s, %s, %s, %s, %s, true, NOW())""",
                (new_sp_id, new_user_id, PROD_BRANCH_ID, xls_name, emp_code, '0000000000', email)
            )
            sp_name_to_id[xls_name] = new_sp_id
            print(f"  CREATED: '{xls_name}' -> {new_sp_id} ({emp_code}) | login: {email}")

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
        """UPDATE sales_orders so SET salesperson_id = %s
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
    print(f"  Not found in DB ({len(not_in_db)}): {not_in_db[:10]}")

# ============================================================
# STEP 4: Delete POS1xxx invoices
# ============================================================
print("\n" + "=" * 60)
print("STEP 4: Deleting POS1xxx invoices")

cur.execute("SELECT id, sales_order_id FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
rows = cur.fetchall()
inv_ids = [r[0] for r in rows]
so_ids = [r[1] for r in rows if r[1]]

print(f"  Found {len(inv_ids)} POS1xxx invoices, {len(so_ids)} linked orders")

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

conn.commit()
print("\n" + "=" * 60)
print("ALL CHANGES COMMITTED TO PRODUCTION.")

# ============================================================
# Verify
# ============================================================
print()
cur.execute("SELECT COUNT(*) FROM sales_invoices")
print("Total invoices now:", cur.fetchone()[0])
cur.execute("SELECT COUNT(*) FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
print("POS1xxx remaining:", cur.fetchone()[0])
cur.execute("SELECT COUNT(*) FROM sales_orders WHERE salesperson_id IS NOT NULL")
print("Orders with salesperson_id:", cur.fetchone()[0])

print()
cur.execute("""SELECT sp.name, COUNT(*) FROM sales_persons sp
    JOIN sales_orders so ON so.salesperson_id = sp.id
    JOIN sales_invoices si ON si.sales_order_id = so.id
    GROUP BY sp.name ORDER BY COUNT(*) DESC""")
print("Salesperson invoice counts:")
for r in cur.fetchall(): print(f"  {r[0]}: {r[1]}")

conn.close()

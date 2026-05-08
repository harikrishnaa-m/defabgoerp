import psycopg2

conn = psycopg2.connect(host='aws-1-ap-southeast-1.pooler.supabase.com', port=5432,
    user='postgres.erjlvvznszwlxqdiduhq', password='ttH1TANuubY3JoMb', dbname='postgres', sslmode='require')
cur = conn.cursor()

import psycopg2

conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com', port=5432,
    user='postgres.erjlvvznszwlxqdiduhq', password='ttH1TANuubY3JoMb',
    dbname='postgres', sslmode='require'
)
cur = conn.cursor()

cur.execute("SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name='users' ORDER BY ordinal_position")
print('=== users table schema ===')
for row in cur.fetchall(): print(row)

print()
cur.execute("SELECT table_schema, table_name FROM information_schema.tables WHERE table_name='users'")
print('users tables by schema:', cur.fetchall())

print()
cur.execute('SELECT id, name, email, password_hash, branch_id, is_active FROM users LIMIT 5')
print('=== existing users ===')
for row in cur.fetchall(): print(row)

print()
cur.execute("SELECT id, name, employee_code, user_id FROM sales_persons WHERE employee_code IN ('SP010','SP011','SP012','SP013','SP014','SP015','SP016')")
print('=== 7 migrated salespersons ===')
for row in cur.fetchall(): print(row)

conn.close()

print()
cur.execute("SELECT count(*) FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
print('POS1xxx count:', cur.fetchone()[0])

cur.execute("SELECT invoice_number FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$' ORDER BY invoice_number")
print('POS1xxx invoices:', [r[0] for r in cur.fetchall()])

conn.close()

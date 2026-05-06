"""Check production returns schema"""
import psycopg2

PROD_DB = dict(
    host='aws-1-ap-south-1.pooler.supabase.com', port=5432,
    user='postgres.zhatpmhkegzyrrmtdzhm', password='Defab_staging_123',
    dbname='postgres', sslmode='require'
)

conn = psycopg2.connect(**PROD_DB)
cur = conn.cursor()

cur.execute("SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name LIKE '%return%' ORDER BY table_name")
print('Return-related tables:', [r[0] for r in cur.fetchall()])
print()

q = 'SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema=%s AND table_name=%s ORDER BY ordinal_position'
for tbl in ['return_orders', 'return_items', 'return_payments', 'ecom_returns']:
    cur.execute(q, ('public', tbl))
    rows = cur.fetchall()
    print(f'=== {tbl} ===')
    for r in rows: print(f'  {r}')
    print()

cur.execute('SELECT COUNT(*) FROM return_orders')
print('Existing return_orders:', cur.fetchone()[0])

conn.close()

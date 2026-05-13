import psycopg2

conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com',
    port=6543,
    user='postgres.erjlvvznszwlxqdiduhq',
    password='ttH1TANuubY3JoMb',
    dbname='postgres',
    sslmode='require'
)
cur = conn.cursor()

# Check schema
for tbl in ['categories', 'variants']:
    cur.execute("SELECT column_name FROM information_schema.columns WHERE table_name=%s ORDER BY ordinal_position", (tbl,))
    print(f'{tbl} cols: {[r[0] for r in cur.fetchall()]}')

# Check the specific mismatched variants in all branches
cur.execute("""
    SELECT v.variant_code, v.price, v.cost_price, p.name as product, c.name as cat
    FROM variants v
    JOIN products p ON p.id = v.product_id
    JOIN categories c ON c.id = p.category_id
    WHERE v.variant_code IN (5903, 6299, 5071, 5067, 1403)
    ORDER BY v.variant_code
""")
rows = cur.fetchall()
print('\nVariants with these codes:')
for r in rows:
    print(r)

cur.close()
conn.close()

"""Verify migration results"""
import psycopg2

conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com', port=5432,
    user='postgres.erjlvvznszwlxqdiduhq', password='ttH1TANuubY3JoMb',
    dbname='postgres', sslmode='require'
)
cur = conn.cursor()

cur.execute("SELECT COUNT(*) FROM sales_invoices")
print("Total invoices now:", cur.fetchone()[0])

cur.execute("SELECT COUNT(*) FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
print("POS1xxx remaining:", cur.fetchone()[0])

cur.execute("SELECT COUNT(*) FROM sales_orders WHERE salesperson_id IS NOT NULL")
print("Orders with salesperson mapped:", cur.fetchone()[0])

cur.execute("SELECT COUNT(*) FROM sales_orders WHERE salesperson_id IS NULL AND channel='POS'")
print("POS Orders still without salesperson:", cur.fetchone()[0])

print()
cur.execute("""
    SELECT sp.name, COUNT(*) as invoice_count
    FROM sales_persons sp
    JOIN sales_orders so ON so.salesperson_id = sp.id
    JOIN sales_invoices si ON si.sales_order_id = so.id
    WHERE sp.branch_id = 'f03469df-dada-4c10-a276-0a4d17416680'
    GROUP BY sp.name ORDER BY invoice_count DESC
""")
print("Salesperson invoice counts (TPTRA branch):")
for row in cur.fetchall():
    print(f"  {row[0]}: {row[1]}")

conn.close()

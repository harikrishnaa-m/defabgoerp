"""Delete POS1xxx invoices and related records (re-run after UUID fix)"""
import psycopg2

conn = psycopg2.connect(
    host='aws-1-ap-southeast-1.pooler.supabase.com', port=5432,
    user='postgres.erjlvvznszwlxqdiduhq', password='ttH1TANuubY3JoMb',
    dbname='postgres', sslmode='require'
)
cur = conn.cursor()

cur.execute("SELECT id, sales_order_id FROM sales_invoices WHERE invoice_number ~ '^POS1[0-9]{3}$'")
rows = cur.fetchall()
inv_ids = [r[0] for r in rows]
so_ids = [r[1] for r in rows if r[1] is not None]

print(f"Found {len(inv_ids)} POS1xxx invoices, {len(so_ids)} linked sales_orders")

if inv_ids:
    cur.execute("DELETE FROM sales_invoice_items WHERE sales_invoice_id = ANY(%s::uuid[])", (inv_ids,))
    print(f"Deleted {cur.rowcount} sales_invoice_items")

    cur.execute("DELETE FROM sales_invoices WHERE id = ANY(%s::uuid[])", (inv_ids,))
    print(f"Deleted {cur.rowcount} sales_invoices")

if so_ids:
    cur.execute("DELETE FROM sales_order_items WHERE sales_order_id = ANY(%s::uuid[])", (so_ids,))
    print(f"Deleted {cur.rowcount} sales_order_items")

    cur.execute("DELETE FROM sales_orders WHERE id = ANY(%s::uuid[])", (so_ids,))
    print(f"Deleted {cur.rowcount} sales_orders")

conn.commit()
print("DONE - all POS1xxx records deleted.")
conn.close()

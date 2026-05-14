import psycopg2
DB = dict(host="aws-1-ap-southeast-1.pooler.supabase.com", port=6543,
          user="postgres.erjlvvznszwlxqdiduhq", password="ttH1TANuubY3JoMb",
          dbname="postgres", sslmode="require")
conn = psycopg2.connect(**DB)
cur  = conn.cursor()

uuids = ['07b2ed52-0b82-4e03-be30-fa335741fbbe', '2696cd55-7906-4a13-986a-e51b871db596']

# Direct price check
for uid in uuids:
    cur.execute("""
        SELECT v.id, v.variant_code, v.price, v.cost_price, p.name, c.name
        FROM variants v
        JOIN products p ON p.id = v.product_id
        JOIN categories c ON c.id = p.category_id
        WHERE v.id = %s::uuid
    """, (uid,))
    r = cur.fetchone()
    if r:
        print(f"id={r[0]}  code={r[1]}  price={r[2]}  cp={r[3]}  item={r[4]}  cat={r[5]}")
    else:
        print(f"UUID {uid} NOT FOUND in variants")

print()
# Stock/branch association
for uid in uuids:
    cur.execute("""
        SELECT s.warehouse_id, w.name, b.name, s.quantity
        FROM stocks s
        JOIN warehouses w ON w.id = s.warehouse_id
        JOIN branches b ON b.id = w.branch_id
        WHERE s.variant_id = %s::uuid
    """, (uid,))
    rows = cur.fetchall()
    if rows:
        for r in rows:
            print(f"variant {uid[:8]}...  wh={r[1]!r}  branch={r[2]!r}  qty={r[3]}")
    else:
        print(f"variant {uid[:8]}... has NO stock rows")

print()
# Also: how many variants with code 2910 or 1421 exist total
for code in [2910, 1421]:
    cur.execute("""
        SELECT v.id, v.variant_code, v.price, p.name, c.name,
               COALESCE(b.name, 'NO STOCK') as branch
        FROM variants v
        JOIN products p ON p.id = v.product_id
        JOIN categories c ON c.id = p.category_id
        LEFT JOIN stocks s ON s.variant_id = v.id
        LEFT JOIN warehouses w ON w.id = s.warehouse_id
        LEFT JOIN branches b ON b.id = w.branch_id
        WHERE v.variant_code = %s
    """, (code,))
    print(f"All variants with code={code}:")
    for r in cur.fetchall():
        print(f"  id={r[0]}  price={r[2]}  item={r[3]}  cat={r[4]}  branch={r[5]}")

cur.close()
conn.close()

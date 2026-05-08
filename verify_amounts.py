"""Investigate net amount mismatches - check Excel AdvAdj/SalesReturnAdj and DB round_off/bill_discount"""
import psycopg2
import pandas as pd

PROD_DB = dict(
    host='aws-1-ap-south-1.pooler.supabase.com', port=5432,
    user='postgres.zhatpmhkegzyrrmtdzhm', password='Defab_staging_123',
    dbname='postgres', sslmode='require'
)
XLS_PATH = 'internal/migration/SalesMigration/Sales Report List.xlsx'

df = pd.read_excel(XLS_PATH, sheet_name='Sheet1', header=0, engine='calamine')
df.columns = ['SlNo','Customer','InvNo','Date','Item','HSN','Rate','Qty','Taxable','TaxRate',
              'CGST','SGST','IGST','TotalGST','InvVal','RoundOff','SalesReturnAdj','AdvAdj',
              'Cheque','Online','Cash','Debitcard','CreditCard']
pos_df = df[df['InvNo'].astype(str).str.match(r'^POS[0-9]{4}$')]
pos_df = pos_df[~pos_df['InvNo'].astype(str).str.match(r'^POS1[0-9]{3}$')]

excel = pos_df.groupby('InvNo').agg(
    taxable=('Taxable','sum'),
    gst=('TotalGST','sum'),
    inv_val=('InvVal','sum'),
    round_off=('RoundOff','sum'),
    sales_return_adj=('SalesReturnAdj','sum'),
    adv_adj=('AdvAdj','sum'),
).reset_index()
excel = excel.round(2)

conn = psycopg2.connect(**PROD_DB)
cur = conn.cursor()
cur.execute("""SELECT invoice_number, sub_amount, gst_amount, round_off, bill_discount, net_amount
    FROM sales_invoices
    WHERE invoice_number ~ '^POS[0-9]{4}$' AND invoice_number !~ '^POS1[0-9]{3}$'""")
rows = cur.fetchall()
conn.close()

db = pd.DataFrame(rows, columns=['invoice_number','sub_amount','gst_amount','db_round_off','bill_discount','net_amount'])
for col in ['sub_amount','gst_amount','db_round_off','bill_discount','net_amount']:
    db[col] = db[col].astype(float).round(2)

merged = pd.merge(excel, db, left_on='InvNo', right_on='invoice_number')
merged['net_diff'] = (merged['inv_val'] - merged['net_amount']).round(2)
merged['gross_excel'] = (merged['taxable'] + merged['gst']).round(2)
merged['gross_db'] = (merged['sub_amount'] + merged['gst_amount']).round(2)
merged['gross_diff'] = (merged['gross_excel'] - merged['gross_db']).round(2)
merged['excel_adj'] = (merged['sales_return_adj'] + merged['adv_adj']).round(2)

mismatches = merged[merged['net_diff'].abs() > 1.0].sort_values('net_diff')
print(f"Net mismatches (>1.0): {len(mismatches)}")
print()
print(f"{'InvNo':<10} {'Excel_net':>10} {'DB_net':>10} {'Diff':>8} | {'Gross_Diff':>10} | {'Excel_Adj(ret+adv)':>19} {'DB_BillDisc':>12}")
print("-" * 90)
for _, r in mismatches.iterrows():
    print(f"{r['InvNo']:<10} {r['inv_val']:>10.2f} {r['net_amount']:>10.2f} {r['net_diff']:>8.2f} | {r['gross_diff']:>10.2f} | {r['excel_adj']:>19.2f} {r['bill_discount']:>12.2f}")

print()
print("Summary:")
print(f"  Excel total SalesReturnAdj:  {excel['sales_return_adj'].sum():.2f}")
print(f"  Excel total AdvAdj:          {excel['adv_adj'].sum():.2f}")
print(f"  DB total bill_discount:      {db['bill_discount'].sum():.2f}")
print()
print(f"  Excel gross total (tax+gst): {(excel['taxable']+excel['gst']).sum():.2f}")
print(f"  DB    gross total (sub+gst): {(db['sub_amount']+db['gst_amount']).sum():.2f}")
print(f"  Gross diff:                  {((excel['taxable']+excel['gst']).sum()-(db['sub_amount']+db['gst_amount']).sum()):.2f}")
print()
print(f"  Excel InvVal total:          {excel['inv_val'].sum():.2f}")
print(f"  DB net_amount total:         {db['net_amount'].sum():.2f}")
print(f"  Net diff:                    {(excel['inv_val'].sum()-db['net_amount'].sum()):.2f}")

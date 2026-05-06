"""
Create placeholder user accounts for the 7 migrated salespersons
and link them via sales_persons.user_id
"""
import psycopg2
import uuid
import bcrypt

DB = dict(
    host='aws-1-ap-southeast-1.pooler.supabase.com', port=5432,
    user='postgres.erjlvvznszwlxqdiduhq', password='ttH1TANuubY3JoMb',
    dbname='postgres', sslmode='require'
)
TPTRA_BRANCH_ID = 'f03469df-dada-4c10-a276-0a4d17416680'

# Placeholder password: Tptra@2024  (they can be changed later via the app)
PLACEHOLDER_PASSWORD = 'Tptra@2024'
password_hash = bcrypt.hashpw(PLACEHOLDER_PASSWORD.encode(), bcrypt.gensalt(rounds=12)).decode()

conn = psycopg2.connect(**DB)
cur = conn.cursor()

# Fetch the 7 salespersons
cur.execute("""SELECT id, name, employee_code, user_id 
               FROM sales_persons WHERE employee_code IN ('SP010','SP011','SP012','SP013','SP014','SP015','SP016')
               ORDER BY employee_code""")
salespersons = cur.fetchall()
print(f"Found {len(salespersons)} salespersons to fix\n")

for sp_id, sp_name, emp_code, existing_user_id in salespersons:
    if existing_user_id:
        print(f"  SKIP {emp_code} '{sp_name}' - already has user_id: {existing_user_id}")
        continue

    email = f"{emp_code.lower()}@tptra.migration"

    # Check if user with this email already exists
    cur.execute("SELECT id FROM users WHERE email = %s", (email,))
    row = cur.fetchone()
    if row:
        user_id = row[0]
        print(f"  EXISTING user found for '{sp_name}': {user_id}")
    else:
        user_id = str(uuid.uuid4())
        cur.execute(
            """INSERT INTO users (id, name, email, password_hash, branch_id, is_active, employee_code, created_at)
               VALUES (%s, %s, %s, %s, %s, true, %s, NOW())""",
            (user_id, sp_name, email, password_hash, TPTRA_BRANCH_ID, emp_code)
        )
        print(f"  CREATED user '{sp_name}' ({emp_code}) -> {user_id} | email: {email}")

    # Link user to salesperson
    cur.execute("UPDATE sales_persons SET user_id = %s WHERE id = %s", (user_id, sp_id))

conn.commit()
print("\nAll changes committed.")

# Verify
print()
cur.execute("""SELECT sp.employee_code, sp.name, sp.user_id, u.email, u.is_active
               FROM sales_persons sp LEFT JOIN users u ON u.id = sp.user_id
               WHERE sp.employee_code IN ('SP010','SP011','SP012','SP013','SP014','SP015','SP016')
               ORDER BY sp.employee_code""")
print("Verification:")
for row in cur.fetchall():
    print(f"  {row[0]} | {row[1]} | user_id={'SET' if row[2] else 'NULL'} | email={row[3]} | active={row[4]}")

conn.close()

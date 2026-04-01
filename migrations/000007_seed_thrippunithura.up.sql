-- ============================================================
-- Seed: StoreManager user, Thrippunithura branch & warehouse
-- ============================================================
-- Password: Defab@123  (bcrypt cost 12)
-- Change the hash below if you need a different password.

BEGIN;

-- 1. Ensure StoreManager role exists
INSERT INTO roles (name, permissions)
VALUES ('StoreManager', NULL)
ON CONFLICT (name) DO NOTHING;

-- 2. Create the store-manager user
INSERT INTO users (id, name, email, password_hash, role_id, branch_id, is_active)
VALUES (
    'b1000000-0000-0000-0000-000000000001',
    'Store Manager Thrippunithura',
    'storemanager.thrippunithura@defab.in',
    '$2a$12$LJ3m4ys3Lk0TSwMBQWyJnOgBNRH6HRSqOzLEbniHbKqcF7wFNPxS',  -- Defab@123
    (SELECT id FROM roles WHERE name = 'StoreManager'),
    NULL,
    TRUE
);

-- 3. Create the branch
INSERT INTO branches (id, name, address, manager_id, branch_code, city, state, phone_number)
VALUES (
    'c1000000-0000-0000-0000-000000000001',
    'DEFAB Thrippunithura',
    'Thrippunithura, Ernakulam, Kerala',
    'b1000000-0000-0000-0000-000000000001',   -- store manager created above
    'BR002',
    'Thrippunithura',
    'Kerala',
    NULL
);

-- 4. Assign the branch back to the user
UPDATE users
SET branch_id = 'c1000000-0000-0000-0000-000000000001'
WHERE id = 'b1000000-0000-0000-0000-000000000001';

-- 5. Create the warehouse under the new branch
INSERT INTO warehouses (id, branch_id, name, type, warehouse_code)
VALUES (
    'd1000000-0000-0000-0000-000000000001',
    'c1000000-0000-0000-0000-000000000001',
    'DEFAB Thrippunithura',
    'STORE',
    'WH002'
);

COMMIT;

DROP INDEX IF EXISTS idx_attendance_user_branch_date;

-- Restore original unique constraint (may fail if duplicate data exists)
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_user_date ON attendance(user_id, date);

ALTER TABLE attendance DROP COLUMN IF EXISTS department;
ALTER TABLE attendance DROP COLUMN IF EXISTS shift;
ALTER TABLE attendance DROP COLUMN IF EXISTS source;
ALTER TABLE attendance DROP COLUMN IF EXISTS session_seq;
ALTER TABLE users DROP COLUMN IF EXISTS employee_code;

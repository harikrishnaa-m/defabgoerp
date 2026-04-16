-- Support multiple punch sessions per day and per branch
-- Allows employees to punch in at different branches on the same day
-- Enables Excel upload of attendance data by branch managers

-- Add session sequence number for multiple punches per day
ALTER TABLE attendance ADD COLUMN IF NOT EXISTS session_seq SMALLINT NOT NULL DEFAULT 1;

-- Track whether attendance was recorded via self-punch or excel upload
ALTER TABLE attendance ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'self';

-- Shift and department from biometric system
ALTER TABLE attendance ADD COLUMN IF NOT EXISTS shift VARCHAR(20) DEFAULT '';
ALTER TABLE attendance ADD COLUMN IF NOT EXISTS department VARCHAR(100) DEFAULT '';

-- Add employee_code to users for biometric system mapping
ALTER TABLE users ADD COLUMN IF NOT EXISTS employee_code VARCHAR(50);

-- Drop old unique constraint that blocks multi-session and multi-branch
DROP INDEX IF EXISTS idx_attendance_user_date;

-- Performance index for common queries
CREATE INDEX IF NOT EXISTS idx_attendance_user_branch_date
    ON attendance(user_id, branch_id, date);

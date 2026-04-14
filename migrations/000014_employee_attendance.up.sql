-- Employee & Attendance module

-- Seed Employee role (idempotent)
INSERT INTO roles (name, permissions)
VALUES ('Employee', '{}')
ON CONFLICT (name) DO NOTHING;

-- Attendance tracking
CREATE TABLE IF NOT EXISTS attendance (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    branch_id   UUID REFERENCES branches(id),
    date        DATE NOT NULL DEFAULT CURRENT_DATE,
    punch_in    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    punch_out   TIMESTAMPTZ,
    total_hours DECIMAL(5,2) DEFAULT 0,
    notes       TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attendance_user ON attendance(user_id);
CREATE INDEX IF NOT EXISTS idx_attendance_branch ON attendance(branch_id);
CREATE INDEX IF NOT EXISTS idx_attendance_date ON attendance(date);
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_user_date ON attendance(user_id, date);

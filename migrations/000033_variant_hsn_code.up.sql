-- Add hsn_code column to variants so Direct GRN can record HSN on new variants
ALTER TABLE variants ADD COLUMN IF NOT EXISTS hsn_code VARCHAR(20);

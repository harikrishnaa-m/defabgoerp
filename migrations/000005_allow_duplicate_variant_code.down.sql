-- Restore unique constraint on variant_code
DROP INDEX IF EXISTS idx_variants_variant_code;
ALTER TABLE variants ADD CONSTRAINT variants_variant_code_key UNIQUE (variant_code);

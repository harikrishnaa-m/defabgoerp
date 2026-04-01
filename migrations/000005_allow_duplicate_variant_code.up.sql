-- Allow duplicate variant_codes (needed for xlsx import where same code can exist in different categories)
-- SKU and barcode remain unique — they are the true identifiers for billing

-- Drop the unique index on variant_code
DROP INDEX IF EXISTS idx_variants_variant_code;
ALTER TABLE variants DROP CONSTRAINT IF EXISTS variants_variant_code_key;
ALTER TABLE variants DROP CONSTRAINT IF EXISTS uni_variants_variant_code;

-- Add a regular (non-unique) index for fast lookups
CREATE INDEX IF NOT EXISTS idx_variants_variant_code ON variants(variant_code);

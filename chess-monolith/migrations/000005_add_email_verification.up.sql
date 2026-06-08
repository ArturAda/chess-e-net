ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verification_code_hash VARCHAR(255);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verification_expires_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMP WITH TIME ZONE;

UPDATE users
SET email_verified = TRUE,
    email_verified_at = COALESCE(email_verified_at, updated_at, CURRENT_TIMESTAMP)
WHERE email_verified = FALSE
  AND email_verification_code_hash IS NULL;

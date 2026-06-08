ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verification_expires_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verification_code_hash;

ALTER TABLE users
    DROP COLUMN IF EXISTS email_verified;

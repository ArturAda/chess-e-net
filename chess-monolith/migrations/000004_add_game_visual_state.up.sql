ALTER TABLE games
    ADD COLUMN IF NOT EXISTS white_visual_state TEXT DEFAULT '{}';

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS black_visual_state TEXT DEFAULT '{}';

UPDATE games
SET white_visual_state = '{}'
WHERE white_visual_state IS NULL OR btrim(white_visual_state) = '';

UPDATE games
SET black_visual_state = '{}'
WHERE black_visual_state IS NULL OR btrim(black_visual_state) = '';

ALTER TABLE games
    ALTER COLUMN white_visual_state SET NOT NULL;

ALTER TABLE games
    ALTER COLUMN black_visual_state SET NOT NULL;

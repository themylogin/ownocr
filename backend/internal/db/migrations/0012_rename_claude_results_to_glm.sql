-- The Ultra rescan read with Claude Opus 5 until GLM-5.3 Flash replaced it. Its
-- readings become GLM readings in place, so the row ids stay and every page keeps
-- the reading a reader chose.
--
-- No `glm` row exists before this runs, because the code that writes one ships
-- with it, so neither rename can collide on page_ocr_page_model_idx.
UPDATE page_ocr
SET model = 'glm',
    data = jsonb_set(data, '{model}', '"z-ai/glm-5.3-flash"')
WHERE model = 'claude';

-- A merged reading names the engine of every line, so the rename reaches inside
-- it. A line the line engine filled keeps `v6`.
UPDATE page_ocr
SET model = 'glm_merged',
    data = jsonb_set(data, '{lines}', (
        SELECT coalesce(jsonb_agg(
            CASE
                WHEN line->>'source' = 'claude' THEN jsonb_set(line, '{source}', '"glm"')
                ELSE line
            END
            ORDER BY position), '[]'::jsonb)
        FROM jsonb_array_elements(data->'lines') WITH ORDINALITY AS item(line, position)
    ))
WHERE model = 'claude_merged';

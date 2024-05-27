BEGIN;

-- The existing 'pontusx' runtime was renamed to 'pontusx_dev', so we rename the existing enum value 
-- to migrate 'pontusx_dev' data. We then re-add the 'pontusx' runtime which corresponds to
-- the new 'pontusx_test' runtime. Note that we do not name this 'pontusx_test' because Nexus
-- is chain-agnostic by design and we do not want to hardcode chain-specific information.
ALTER TYPE public.runtime RENAME VALUE 'pontusx' TO 'pontusx_dev';
ALTER TYPE public.runtime ADD VALUE IF NOT EXISTS 'pontusx';

UPDATE analysis.processed_blocks
SET 
    analyzer = 'pontusx_dev'
WHERE
    analyzer = 'pontusx';

COMMIT;

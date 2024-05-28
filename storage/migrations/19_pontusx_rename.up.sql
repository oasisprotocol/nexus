BEGIN;

-- The new 'pontusx_test' runtime was initially tracked as 'pontusx' in the database. We 
-- rename the existing enum value to migrate 'pontusx' data to 'pontusx_test'. 
ALTER TYPE public.runtime RENAME VALUE 'pontusx' TO 'pontusx_test';

UPDATE analysis.processed_blocks
SET 
    analyzer = 'pontusx_test'
WHERE
    analyzer = 'pontusx';

COMMIT;

BEGIN;

ALTER TABLE chain.entities
    ADD start_date TIMESTAMP WITH TIME ZONE;

WITH entity_starts AS (
    SELECT entities.address, MIN(blocks.time) AS start
    FROM chain.entities AS entities 
    LEFT JOIN chain.transactions AS txs ON 
        entities.address = txs.sender AND 
        txs.method = 'registry.RegisterEntity'
    JOIN chain.blocks AS blocks ON txs.block = blocks.height
    GROUP BY entities.address
)
UPDATE chain.entities
SET
    start_date = entity_starts.start
FROM entity_starts
WHERE chain.entities.address = entity_starts.address;

COMMIT;

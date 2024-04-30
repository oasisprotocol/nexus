BEGIN;

ALTER TABLE chain.entities
    ADD start_block UINT63;

-- Backfill entities.start_block
WITH entity_starts AS (
    SELECT entities.address, MIN(txs.block) AS start_height
    FROM chain.entities AS entities 
    LEFT JOIN chain.transactions AS txs ON 
        entities.address = txs.sender AND 
        txs.method = 'registry.RegisterEntity' AND
        txs.code = 0 -- ignore failed txs
    GROUP BY entities.address
)
UPDATE chain.entities
SET
    start_block = entity_starts.start_height
FROM entity_starts
WHERE chain.entities.address = entity_starts.address;

COMMIT;

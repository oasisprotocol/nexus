BEGIN;

ALTER TABLE chain.runtime_events 
    ADD COLUMN TIMESTAMP timestamp WITH time zone; 

UPDATE chain.runtime_events AS evs
    SET    timestamp = blocks.timestamp
    FROM   chain.runtime_blocks AS blocks
    WHERE  evs.runtime = blocks.runtime
       AND evs.round = blocks.round;

ALTER TABLE chain.runtime_events
    ALTER COLUMN timestamp
        SET NOT NULL;

COMMIT;

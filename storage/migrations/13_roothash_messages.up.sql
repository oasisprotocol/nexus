BEGIN;

-- Roothash messages are small structures that a runtime can send to
-- communicate with the consensus layer. They are agreed upon for each runtime
-- block. We'll see the messages themselves in the proposal for that block,
-- i.e. in the first executor commit for the round. The consensus layer
-- processes these messages when the block gets finalized, which produces a
-- result for each message.
--
-- In Cobalt and below, the roothash consensus app emits an event for each
-- message. In Damask and up, the results are stored on chain, and you use a
-- roothash "get last round results" query to look up the results.
--
-- This table has tracked runtimes' messages and results. Either of the
-- message or result may be absent, as they can be disseminated in different
-- consensus blocks.

CREATE TABLE chain.roothash_messages (
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    message_index UINT31 NOT NULL,
    PRIMARY KEY (runtime, round, message_index),
    type TEXT,
    body JSONB,
    error_module TEXT,
    error_code UINT31,
    result BYTEA,
    related_accounts oasis_addr[]
);

COMMIT;

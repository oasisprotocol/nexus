BEGIN;

-- Tracks the current (consensus) height of the node. TODO: expand to paratime heights.
CREATE TABLE chain.latest_node_heights
(
  layer TEXT NOT NULL PRIMARY KEY,
  height UINT63 NOT NULL
);

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;

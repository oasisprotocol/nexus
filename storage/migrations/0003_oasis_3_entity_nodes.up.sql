-- Entity Node relationship migration.
--
-- Entities are free to claim any node, and nodes are free to associate
-- themselves with any entity. So this relationship may not necessarily
-- be perfectly associated as is currently assumed.

ALTER TABLE oasis_3.nodes DROP CONSTRAINT nodes_entity_id_fkey;

CREATE TABLE IF NOT EXISTS oasis_3.claimed_nodes
(
  entity_id TEXT NOT NULL,
  node_id   TEXT NOT NULL,

  PRIMARY KEY (entity_id, node_id)  
);

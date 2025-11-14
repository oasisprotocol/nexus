BEGIN;

CREATE TYPE public.node_role AS ENUM ('compute', 'observer', 'key-manager', 'validator', 'consensus-rpc', 'storage-rpc');

ALTER TABLE chain.nodes
  ADD COLUMN IF NOT EXISTS roles_new node_role[] NOT NULL DEFAULT '{}'::node_role[];

-- Migrate existing comma-separated string roles to the new enum array column.
WITH
  split AS (
    SELECT
      id,
      regexp_split_to_table(COALESCE(roles, ''), '\s*,\s*') AS role_name
    FROM
      chain.nodes
    WHERE
      roles IS NOT NULL
      AND roles <> ''
  ),
  filtered AS (
    SELECT
      id,
      role_name
    FROM
      split
    WHERE
      role_name IN (
        'validator',
        'compute',
        'key-manager',
        'storage-rpc',
        'observer',
        'consensus-rpc'
      )
  ),
  dedup AS (
    SELECT DISTINCT
      id,
      role_name::node_role AS val
    FROM
      filtered
  ),
  agg AS (
    SELECT
      id,
      array_agg(val ORDER BY val) AS arr
    FROM
      dedup
    GROUP BY
      id
  )
UPDATE
  chain.nodes AS n
SET
  roles_new = a.arr
FROM
  agg AS a
WHERE
  n.id = a.id;

ALTER TABLE chain.nodes DROP COLUMN roles;
ALTER TABLE chain.nodes RENAME COLUMN roles_new TO roles;

COMMIT;

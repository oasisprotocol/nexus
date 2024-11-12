BEGIN;

CREATE MATERIALIZED VIEW views.validator_uptimes AS
	-- With a limit of 14400 blocks, this is the last ~24 hrs of signatures.
	WITH last_window_blocks AS (
		SELECT height, signer_entity_ids
		FROM chain.blocks
		ORDER BY height DESC
		LIMIT 14400
		OFFSET 1 -- Omit the most recent block; signatures for it are obtained in the next block.
	),
	-- Generate a series of 12 segments representing ~2 hours within the window.
	all_segments AS (
		SELECT generate_series(0, 11) AS segment_id
	),
	-- Segments of blocks of ~2 hours within the main window, with expanded signers.
	segment_blocks AS (
		SELECT
			height,
			UNNEST(signer_entity_ids) AS signer_entity_id,
			(ROW_NUMBER() OVER (ORDER BY height DESC) - 1) / 1200 AS segment_id
		FROM last_window_blocks
	),
	-- Count signed blocks in each segment.
	segment_counts AS (
		SELECT
			signer_entity_id,
			segment_id,
			COUNT(height) AS signed_blocks_count
		FROM
			segment_blocks
		-- Compute this for all validators; the client can select from the view if needed.
		GROUP BY
			signer_entity_id, segment_id
	)
	-- Group windows per signer and calculate overall percentage.
	SELECT
		signers.signer_entity_id AS signer_entity_id,
		COALESCE(SUM(signed_blocks_count), 0) AS window_signed,
		ARRAY_AGG(COALESCE(segment_counts.signed_blocks_count, 0) ORDER BY segment_counts.segment_id) AS segments_signed,
		14400 AS window_length, -- 14400 blocks per window.
		1200 AS segment_length -- 1200 blocks per segment.
	FROM
		-- Ensure we have all windows for each signer, even if they didn't sign in a particular window.
		(SELECT DISTINCT signer_entity_id FROM segment_counts) AS signers
		CROSS JOIN all_segments
		LEFT JOIN segment_counts ON signers.signer_entity_id = segment_counts.signer_entity_id AND all_segments.segment_id = segment_counts.segment_id
	GROUP BY
		signers.signer_entity_id;

CREATE UNIQUE INDEX ix_views_validator_uptimes_signer_entity_id ON views.validator_uptimes (signer_entity_id); -- A unique index is required for CONCURRENTLY refreshing the view.

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA views TO PUBLIC;

END;

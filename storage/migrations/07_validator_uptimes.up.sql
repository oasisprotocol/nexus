BEGIN;

CREATE MATERIALIZED VIEW views.validator_uptimes AS
	-- With a limit of 14400 blocks, this is the last ~24 hrs of signatures.
	WITH last_window_blocks AS (
		SELECT height, signer_entity_ids
		FROM chain.blocks
		ORDER BY height DESC
		LIMIT 14400
	),
	-- Generate a series of 12 slots representing ~2 hours within the window.
	all_slots AS (
		SELECT generate_series(0, 11) AS slot_id
	),
	-- Slots of blocks of ~2 hours within the main window, with expanded signers.
	slot_blocks AS (
		SELECT
			height,
			UNNEST(signer_entity_ids) AS signer_entity_id,
			(ROW_NUMBER() OVER (ORDER BY height DESC) - 1) / 1200 AS slot_id
		FROM last_window_blocks
	),
	-- Count signed blocks in each slot.
	slot_counts AS (
		SELECT
			signer_entity_id,
			slot_id,
			COUNT(height) AS signed_blocks_count
		FROM
			slot_blocks
		-- Compute this for all validators; the client can select from the view if needed.
		GROUP BY
			signer_entity_id, slot_id
	)
	-- Group windows per signer and calculate overall percentage.
	SELECT
		signers.signer_entity_id,
		ARRAY_AGG(COALESCE(slot_counts.signed_blocks_count, 0) ORDER BY slot_counts.slot_id) AS slot_signed,
		COALESCE(SUM(signed_blocks_count), 0) AS overall_signed
	FROM
		-- Ensure we have all windows for each signer, even if they didn't sign in a particular window.
		(SELECT DISTINCT signer_entity_id FROM slot_counts) AS signers
		CROSS JOIN all_slots
		LEFT JOIN slot_counts ON signers.signer_entity_id = slot_counts.signer_entity_id AND all_slots.slot_id = slot_counts.slot_id
	GROUP BY
		signers.signer_entity_id;

CREATE UNIQUE INDEX ix_views_validator_uptimes_signer_entity_id ON views.validator_uptimes (signer_entity_id); -- A unique index is required for CONCURRENTLY refreshing the view.

END;

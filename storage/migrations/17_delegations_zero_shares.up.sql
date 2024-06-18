BEGIN;

-- Starting with this commit, the delegations with zero shares will get cleaned up
-- at the debond start time (on reclaim escrow transaction). Here we clean up the
-- delegations that were already debonded and have 0 shares at the time of this
-- deployment.
DELETE FROM chain.delegations
    WHERE shares = 0;

COMMIT;

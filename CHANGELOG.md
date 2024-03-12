# Change Log

All notable changes to this project are documented in this file.

The format is inspired by [Keep a Changelog].

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/

<!-- markdownlint-disable no-duplicate-heading -->

<!-- NOTE: towncrier will not alter content above the TOWNCRIER line below. -->

<!-- TOWNCRIER -->

## 0.2.9 (2024-02-15)

This release focuses on performance and internal cleanup rather than features. Among other things, it brings efficiency improvements around DB communication (~8x faster txs).

**NOTE:** The bugfix in #628 requires a DB reindex (or careful manual intervention) to apply. However no action is needed if your instance has not yet run the (relatively new) `abi_backfill` analyzer before.

### Improvements and Features
- prometheus: Histograms: Use more buckets by @mitjat in https://github.com/oasisprotocol/nexus/pull/624
- pgx: Transfer entire DB batch in one request by @mitjat in https://github.com/oasisprotocol/nexus/pull/556
- pgx: fast batches: always create an explicit tx by @mitjat in https://github.com/oasisprotocol/nexus/pull/636
- Precompute runtime account stats (total_sent, total_received) by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/618

### Bugfixes
- bugfix: abi_backfill: key event updates by event body by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/628
- consensus: remove separator comment queries by @pro-wh in https://github.com/oasisprotocol/nexus/pull/629
- make evm_log_* fields null for non evm.log events by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/640

### Internal Changes
- Sync gitlint config with other repos by @buberdds in https://github.com/oasisprotocol/nexus/pull/625
- usable changes from roothash work by @pro-wh in https://github.com/oasisprotocol/nexus/pull/631

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.8...v0.2.9

## 0.2.8 (2024-02-04)

### Little things
- pogreb: Remove .bac.bac files manually by @mitjat in https://github.com/oasisprotocol/nexus/pull/620
- PR 620 follow-up: tweaks in comments by @mitjat in https://github.com/oasisprotocol/nexus/pull/623
- optimize daily active accounts query by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/622

### Internal changes
- Bump golang.org/x/crypto (0.14->0.18), metadata-registry-tools by @mitjat in https://github.com/oasisprotocol/nexus/pull/614
- build(deps): bump the go_modules group across 1 directories with 2 updates by @dependabot in https://github.com/oasisprotocol/nexus/pull/615
- logging: Use global logging config everywhere by @mitjat in https://github.com/oasisprotocol/nexus/pull/621

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.7...v0.2.8

## 0.2.7 (2024-01-25)

### What's Changed
- analyzer/runtime: fast-sync: do not update common pool, fee accumulator by @mitjat in https://github.com/oasisprotocol/nexus/pull/593
- api: enum proposal states by @lukaw3d in https://github.com/oasisprotocol/nexus/pull/612
- statecheck: exempt accounts that are stale (to be requeried) by @ptrus in https://github.com/oasisprotocol/nexus/pull/611
- bugfix: abi analyzer no longer overwrites error_message on failure (coalesce version) by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/613

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.6...v0.2.7

## 0.2.6 (2024-01-22)

### Manual database fixes needed here
This release contains fixes to previous migrations. Manually make changes to your database to match. See the following for these changes.

- db: Make eth_preimage() et al visible to api layer by @mitjat in https://github.com/oasisprotocol/nexus/pull/606
- active_accounts query optimization by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/607

### New features
- Andrew7234/abi api by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/586

### Bug fixes
- analyzer/block: Preemptively terminate batch before timeout by @mitjat in https://github.com/oasisprotocol/nexus/pull/605
- storage: correct nft id type again by @pro-wh in https://github.com/oasisprotocol/nexus/pull/609

### Internal changes
- [metrics] add block analyzer queuelength metrics by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/577
- e2e_regression: Manage RPC cache with git-lfs by @mitjat in https://github.com/oasisprotocol/nexus/pull/608

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.5...v0.2.6

## 0.2.5 (2024-01-16)

### New features
- api: add NFT-oriented event filtering by @pro-wh in https://github.com/oasisprotocol/nexus/pull/579
- API: add NFT num_transfers by @pro-wh in https://github.com/oasisprotocol/nexus/pull/585

### Bug fixes
- storage: avoid nil dereference when closing uninitialized kvstore by @pro-wh in https://github.com/oasisprotocol/nexus/pull/597
- runtime-transactions: fix calculated `charged_fee` for non-evm txs by @ptrus in https://github.com/oasisprotocol/nexus/pull/602
- storage: correct nft id type by @pro-wh in https://github.com/oasisprotocol/nexus/pull/604

### Internal changes
- tests: require.Nil -> require.NoError by @ptrus in https://github.com/oasisprotocol/nexus/pull/598
- e2e-regression: Use block range in Eden by @mitjat in https://github.com/oasisprotocol/nexus/pull/596
- tests/statecheck/runtime: Report the number of balance discrepancies by @ptrus in https://github.com/oasisprotocol/nexus/pull/600
- statecheck: Add support for sapphire statecheck by @ptrus in https://github.com/oasisprotocol/nexus/pull/603

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.4...v0.2.5

## 0.2.4 (2023-12-23)

### Bug fixes
- runtime: upsert evm contract creation info by @pro-wh in https://github.com/oasisprotocol/nexus/pull/595

### Little things
- metrics: Add timings for data fetch, data analysis by @mitjat in https://github.com/oasisprotocol/nexus/pull/572

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.3...v0.2.4

## 0.2.3 (2023-12-22)

### Bug fixes
- common/types: BigInt: CBOR-serialize correctly by @mitjat in https://github.com/oasisprotocol/nexus/pull/592

### Little things
- Revert "analyzer/block: more efficient db query on startup" by @pro-wh in https://github.com/oasisprotocol/nexus/pull/594

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.2...v0.2.3

## 0.2.2 (2023-12-21)

### Bug fixes
- evmtokenbalances: correct a log field by @pro-wh in https://github.com/oasisprotocol/nexus/pull/591

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.1...v0.2.2

## 0.2.1 (2023-12-21)

### New features
- api: Serve v1 API at root URL "/" by @mitjat in https://github.com/oasisprotocol/nexus/pull/583

### Little things
- analyzer: log item errors by @pro-wh in https://github.com/oasisprotocol/nexus/pull/587

### Bug fixes
- runtime: fix owner update for burned NFT instances by @pro-wh in https://github.com/oasisprotocol/nexus/pull/589
- runtime: add explicit PossibleNFT.Burned field by @pro-wh in https://github.com/oasisprotocol/nexus/pull/590

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.2.0...v0.2.1

## 0.2.0 (2023-12-12)

### Internal Changes
- consolidate migrations by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/574

**Note**
This release is *not* backwards compatible due to the db schema consolidation in 574. We recommend a full db wipe prior to upgrading to this release.

If upgrading an existing deployment, you should first upgrade to `v0.1.26` and then upgrade to `v0.2.0` to ensure that all db migrations are properly applied. Devs will also need to manually adjust the `schema_migrations` table to 5 to ensure that future migrations are properly applied.

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.26...v0.2.0

## 0.1.26 (2023-12-12)

### Internal Changes
- abi analyzer by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/540

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.25...v0.1.26

## 0.1.25 (2023-12-08)

### Spotlight change
- query block by hash by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/581

### New features
- analyzer/consensus: Support ParametersChange proposal by @ptrus in https://github.com/oasisprotocol/nexus/pull/573
- analyzer/runtime: Support WROSE events Deposit, Withdrawal by @mitjat in https://github.com/oasisprotocol/nexus/pull/575

## Little things
- analyzer: damask: Warn about incompatibility with slow-sync by @mitjat in https://github.com/oasisprotocol/nexus/pull/565
- config: Add Eden chain context and genesis heights by @ptrus in https://github.com/oasisprotocol/nexus/pull/576
- Removed hardwired "Emerald" from API docs by @csillag in https://github.com/oasisprotocol/nexus/pull/580
- Improve documentation of the block hash argument by @csillag in https://github.com/oasisprotocol/nexus/pull/582

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.24...v0.1.25

## 0.1.24 (2023-11-22)

### New features

- analyzer/fast-sync: Reduce DB contention by writing updates into a writeahead log by @mitjat in https://github.com/oasisprotocol/nexus/pull/567

### Internal changes

- lint: Remove obsolete linters by @mitjat in https://github.com/oasisprotocol/nexus/pull/569
- common.BigInt.String() and some unprovoked changes by @pro-wh in https://github.com/oasisprotocol/nexus/pull/571

### Bug fixes

- storage: handle NULL NFT owner by @pro-wh in https://github.com/oasisprotocol/nexus/pull/570

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.23...v0.1.24

## 0.1.23 (2023-11-17)

### New features

- parse unencrypted results for confidential txs by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/544

### Bug fixes

- evmnfts, evmtokens: utf-8-clean strings from chain by @pro-wh in https://github.com/oasisprotocol/nexus/pull/568

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.22...v0.1.23

## 0.1.22 (2023-11-14)

### New features

- analyzer/block: more efficient db query on startup by @mitjat in https://github.com/oasisprotocol/nexus/pull/559

### Internal changes

- Rename enigma -> eden by @mitjat in https://github.com/oasisprotocol/nexus/pull/562
- Add "bisect" tool for finding where oasis-node and db diverge by @mitjat in https://github.com/oasisprotocol/nexus/pull/564

### Bug fixes

- runtime/evm: ignore unsupported token types by @pro-wh in https://github.com/oasisprotocol/nexus/pull/563

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.21...v0.1.22

## 0.1.21 (2023-11-07)

### New features

- analyzer: Slow-sync analyzers wait only for _relevant_ fast-sync analyzers to complete by @mitjat in https://github.com/oasisprotocol/nexus/pull/558
- Connect to oasis-node lazily by @mitjat in https://github.com/oasisprotocol/nexus/pull/555

### Internal changes

- refactor: Simplify node client stack (remove `ConsensusSourceStorage`) by @mitjat in https://github.com/oasisprotocol/nexus/pull/554

### Bug fixes

- analyzer: Fix syntax error in SQL (TakeEscrow) by @mitjat in https://github.com/oasisprotocol/nexus/pull/560
- runtime/evm: prevent saving invaild NFT metadata JSON by @pro-wh in https://github.com/oasisprotocol/nexus/pull/561

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.20...v0.1.21

## 0.1.20 (2023-11-03)

### Features

- NFT original metadata by @pro-wh in https://github.com/oasisprotocol/nexus/pull/552

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.19...v0.1.20

## 0.1.19 (2023-11-03)

This release is a hotfix for v0.1.18, for which we were unable to build a Docker image. This release contains a single PR:

- Bump Go (1.21.3), golangci-lint (1.55.1) by @mitjat in https://github.com/oasisprotocol/nexus/pull/557

## v0.1.18 (2023-11-02)

This release adds support for oasis-core v23.0 (currently deployed on testnet 2023-10-12; a dump-restore upgrade to mainnet is upcoming).

### What's Changed

- refactor: Vendor oasis-core v22.2, Use mostly vendored oasis-core by @mitjat in https://github.com/oasisprotocol/nexus/pull/548
- security fix: bump golang.org/x/net by @mitjat in https://github.com/oasisprotocol/nexus/pull/551
- Support node v23.0 by @mitjat in https://github.com/oasisprotocol/nexus/pull/549

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.17...v0.1.18

## 0.1.17 (2023-10-20)

### What's Changed

- storage: move token type translation to go by @pro-wh in https://github.com/oasisprotocol/nexus/pull/533
- NFT APIs by @pro-wh in https://github.com/oasisprotocol/nexus/pull/514
- add testnet history record for 2023-10-12 upgrade by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/547
- NFT transfer tracking by @pro-wh in https://github.com/oasisprotocol/nexus/pull/527
- NFT query by owner by @pro-wh in https://github.com/oasisprotocol/nexus/pull/529
- NFT query by ID by @pro-wh in https://github.com/oasisprotocol/nexus/pull/538
- api: filter NFTs by owner and token addr by @pro-wh in https://github.com/oasisprotocol/nexus/pull/539
- EVM parsing functions with ABI by @pro-wh in https://github.com/oasisprotocol/nexus/pull/421
- runtime: use abiparse in VisitEVMEvent by @pro-wh in https://github.com/oasisprotocol/nexus/pull/521
- analyzer/runtime: Record withdraw.From consensus addr as related to runtime tx by @mitjat in https://github.com/oasisprotocol/nexus/pull/537
- config: add evm_nfts_sapphire by @pro-wh in https://github.com/oasisprotocol/nexus/pull/550

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.16...v0.1.17

## 0.1.16 (2023-10-03)

### What's Changed

Features

- consensus: fast-sync mode: Skip dead reckoning by @mitjat in https://github.com/oasisprotocol/nexus/pull/457
- NFT analyzer by @pro-wh in https://github.com/oasisprotocol/nexus/pull/506
- Andrew7234/tx revert encoding by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/530

Internal/Misc

- README: add tools install instructions by @pro-wh in https://github.com/oasisprotocol/nexus/pull/523
- testing: extend regression tests to item analyzers by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/519
- api: Simplify validator queries (no-op) by @mitjat in https://github.com/oasisprotocol/nexus/pull/524
- Parse encrypted evm.Create txs correctly by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/526
- Update CODEOWNERS by @aefhm in https://github.com/oasisprotocol/nexus/pull/531
- e2e_regression: Increase coverage of tests by @mitjat in https://github.com/oasisprotocol/nexus/pull/525
- bugfix: kvstore: Error messages do not decode the key correctly by @mitjat in https://github.com/oasisprotocol/nexus/pull/534
- kvstore: Add prometheus metric for cache hits/misses by @mitjat in https://github.com/oasisprotocol/nexus/pull/535

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.15...v0.1.16

## 0.1.15 (2023-09-21)

### What's Changed

Hotfix:

- fix evm_tokens query by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/522

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.14...v0.1.15

## 0.1.14 (2023-09-20)

### What's Changed

Features:

- analyzer/runtime: Index (un)delegate events, txs by @mitjat in https://github.com/oasisprotocol/nexus/pull/515

Internal:

- nodeapi: cache deterministic failure responses by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/518
- always update last_download_round for evm_tokens by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/520

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.13...v0.1.14

## 0.1.13 (2023-09-14)

### What's Changed

Features

- api: Change format of latest_update to int64 by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/513
- analyzers, api: Fix dead reckoning of `gas_used`, `num_transfers`, `total_balance` by @mitjat in https://github.com/oasisprotocol/nexus/pull/512

Internal

- add itemBasedAnalyzer; refactor evm analyzers accordingly by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/511
- analyzer/runtime: Register special runtime addresses by @mitjat in https://github.com/oasisprotocol/nexus/pull/516
- bugfix: analyzer/consensus: votes: upsert instead of insert by @mitjat in https://github.com/oasisprotocol/nexus/pull/517

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.12...v0.1.13

## 0.1.12 (2023-08-30)

This release provides compatibility with Sapphire 0.6.0.

### What's Changed

Features:

- Mark charged_fee as required by @lukaw3d in https://github.com/oasisprotocol/nexus/pull/501
- analyzer/runtime/consensusaccounts: Handle new client-sdk 0.6.0 event codes by @mitjat in https://github.com/oasisprotocol/nexus/pull/510

Internal:

- analyzer: rename from0xChecksummed following sdk change by @pro-wh in https://github.com/oasisprotocol/nexus/pull/509

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.11...v0.1.12

## 0.1.11 (2023-08-28)

### What's Changed

Features:

- Andrew7234/evm token transfers by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/504
- Andrew7234/account num txs by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/505
- Ability to run analyzers in parallel by @mitjat in https://github.com/oasisprotocol/nexus/pull/456
- Andrew7234/status info by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/496

Internal:

- Adjust chain.runtime_events indexes by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/500
- bugfix: db: grant permissions on new tables by @mitjat in https://github.com/oasisprotocol/nexus/pull/507
- Bump client-sdk to 0.6.0 (for Sapphire 0.6.0) by @mitjat in https://github.com/oasisprotocol/nexus/pull/508

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.10...v0.1.11

## 0.1.10 (2023-08-16)

This is a minor release **with a (small) breaking API change** (see #477)

### What's Changed

Features:

- tx_volume stats: Produce windowed results, compute more efficiently by @ptrus in https://github.com/oasisprotocol/nexus/pull/477
- Andrew7234/runtime account gas by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/495

Internal

- (in preparation for NTF metadata caching) analyzer: add pubclient by @pro-wh in https://github.com/oasisprotocol/nexus/pull/499
- bugfix: analyzer/nodeapi: Make CommitteeKind compatbile across cobalt and damask by @mitjat in https://github.com/oasisprotocol/nexus/pull/502

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.9...v0.1.10

## 0.1.9 (2023-08-07)

This is a bugfix release. Block 854327 in emerald mainnet is subject to the bug fixed in #497.

### What's Changed

Bugfixes:

- update total_supply to accept negative updates by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/494
- bugfix: initialize possibleToken fields by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/497

Internal:

- pogreb: non-blocking recovery by @ptrus in https://github.com/oasisprotocol/nexus/pull/486
- add index for chain.addres_preimages.address_data by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/498

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.8...v0.1.9

## 0.1.8 (2023-07-31)

### What's Changed

This release primarily brings updates that make it possible for the Explorer web UI to fetch all data in fewer requests (O(1) for all pages, as opposed to O(rows) for some pages)

Features:

- add timestamp and token details to runtime events by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/491
- [api] return evm address instead of oasis addr in RuntimeEvmBalance by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/492
- Andrew7234/evmtokens api by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/493

Bugfixes:

- analyzer: bugfix download erc721 mutable data by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/490

Internal:

- Andrew7234/stress test by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/473
- cmd: Log goroutines (stack traces) on SIGUSR1 by @mitjat in https://github.com/oasisprotocol/nexus/pull/471

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.7...v0.1.8

## 0.1.7 (2023-07-17)

### What's Changed

Features

- Re-enable ERC-721 token type in the API specs by @csillag in https://github.com/oasisprotocol/nexus/pull/478
- analyzer/evm: handle downloading ERC-721 mutable data by @pro-wh in https://github.com/oasisprotocol/nexus/pull/480
- [analyzer] dead reckon erc721 total supply by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/485

Internal

- statecheck/runtime: fix balance checks by @ptrus in https://github.com/oasisprotocol/nexus/pull/476
- statecheck: select latest block that is processed by @ptrus in https://github.com/oasisprotocol/nexus/pull/475
- paratime analyzers other than block analyzer, queue length metric by @pro-wh in https://github.com/oasisprotocol/nexus/pull/482

Fixes/Misc

- bump grpc to v1.53.0 by @pro-wh in https://github.com/oasisprotocol/nexus/pull/474
- Fix typo on name of token standard in comment by @csillag in https://github.com/oasisprotocol/nexus/pull/479
- Fix Discord links to use URL under our control by @lukaw3d in https://github.com/oasisprotocol/nexus/pull/484
- Update CONTRIBUTING.md branch name reference by @aefhm in https://github.com/oasisprotocol/nexus/pull/483
- [db] add index to improve /status queries by @Andrew7234 in https://github.com/oasisprotocol/nexus/pull/481
- Update API spec base URL by @aefhm in https://github.com/oasisprotocol/nexus/pull/488
- Update CONTRIBUTING.md by @aefhm in https://github.com/oasisprotocol/nexus/pull/487

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.6...v0.1.7

## 0.1.6 (2023-07-03)

### What's Changed

Features:

- ERC-721 support by @pro-wh in https://github.com/oasisprotocol/nexus/pull/447
- analyzer/runtime: enable balance tracking for ERC-721 by @pro-wh in https://github.com/oasisprotocol/nexus/pull/459
- /{runtime}/transactions: Add heuristic field for "is tx a native token transfer?" by @mitjat in https://github.com/oasisprotocol/nexus/pull/469

Internal:

- storage: move some wipe logic out to functions by @pro-wh in https://github.com/oasisprotocol/nexus/pull/465

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.5-rc...v0.1.6

## 0.1.5-rc (2023-06-28)

"Indexer" is now "Nexus"!
Minor release. The main motivation for cutting it so to test the auto-release process (CD) after The Big Rename (#467).

### Features

- api: allow filter EVM tokens by name by @pro-wh in https://github.com/oasisprotocol/nexus/pull/460

### Internal

- bugfix: db: Make custom types public by @mitjat in https://github.com/oasisprotocol/nexus/pull/464
- Close IO resources (KVStore!) even if analyzers are slow to shut down by @mitjat in https://github.com/oasisprotocol/nexus/pull/463
- Rename `oasis-indexer` to `nexus` by @mitjat in https://github.com/oasisprotocol/nexus/pull/467

**Full Changelog**: https://github.com/oasisprotocol/nexus/compare/v0.1.4...v0.1.5-rc

## 0.1.4 (2023-06-27)

### Features

- More insights into EVM smart contracts: bytecode and source code:
  - Analyzer for fetching bytecode of EVM contracts by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/448
  - evm/sourcify: support EVM contract verification via Sourcify by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/429
- analyzer: genesis: parse compute nodes for each runtime by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/458
- api: /{runtime}/accounts/{addr}: Show contract runtime bytecode, show eth address of originating tx by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/452
- api: /{runtime}/status: Expose datetime of most recent block by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/462
- api: Add /{runtime}/evm_tokens/{address} endpoint by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/453
- api: Add /evm_tokens/{address}/holders endpoint by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/455

### Internal

- e2e_regression: Fix expected output after #452 by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/454
- db: Move analysis tables to their own schema by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/451

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.1.3...v0.1.4

## 0.1.3 (2023-06-14)

Bugfix release. No DB schema changes.

### What's Changed

- api: EVM tokens: output only tokens of supported types; fix error generation by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/446

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.1.2...v0.1.3

## 0.1.2 (2023-06-13)

Tiny release for tiny UI feature. No DB reindex needed.

### What's Changed

- storage: read token name by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/444

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.1.1...v0.1.2

## 0.1.1 (2023-06-09)

### Breaking changes

This release introduces breaking changes to the DB. Wipe it, or reconstruct a migration by looking at `git diff v0.1.0..v0.1.1 storage/migrations`.

### New features

Machinery for upcoming contract verification:

- Andrew7234/contract accounts by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/428
- evm: only store log signatures for parsed events by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/423

### Performance improvements:

- analyzer: runtime_events evm-tx hash query optimisation by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/443

### Code quality improvements:

- tests: assert.Nil -> require.Nil in some places by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/431
- Andrew7234/e2e extension by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/442

### Fixes

- evm_tokens: fix potential nil-pointer dereference by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/430
- storage: add genesis document chain context parameter by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/433
- tests: don't run tests concurrently by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/438
- cmd: let 'from' height choose genesis doc by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/434
- runtime: error message does not need to be a valid utf-8 by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/440
- evmtokens: check for nil EVMTokenMutableData by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/441

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.1.0...v0.1.1

## 0.1.0 (2023-05-26)

This is the first release that aims to have enough features to support a basic blockchain explorer for Sapphire and Emerald. It has not yet been integrated/tested with the frontend, so we expect follow-up releases.

### Breaking changes

This release introduces breaking changes to the DB. Wipe it, or reconstruct a migration by looking at `git diff v0.0.17..v0.1.0 storage/migrations`.

### Features

Support for encrypted txs (Sapphire):

- show us the encrypted data by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/407
- Andrew7234/feature/envelope api by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/415

Tracking runtime balances that can be changed "silently" by EVM contracts:

- evm_tokens: Explicitly tracks unsupported tokens in the DB by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/416
- evm_token_balances: Refresh balance of native tokens by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/422

Work towards parallelism/speedup (WIP):

- analyzer: grab blocks in parallelism-friendly way by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/389
- analyzer/block: configurable batch size by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/417
- Andrew7234/ci e2e by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/410

Lots of small newly exposed pieces of data:

- /{runtime}/transaction(s): Expose actual fee used by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/420
- /{runtime}/events: Include eth_tx_hash in response by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/419
- API: support filtering (runtime) transactions by timestamp by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/425
- API: add endpoints for querying (debonding)delegations to an address by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/424
- api: Recognize sapphire, cipher as valid Layer values by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/427

### Fixes

- consensus: make malformed transaction not fail by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/418
- stats: Update materialized views concurrently by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/426

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.17...v0.1.0

## 0.0.17 (2023-05-10)

### What's Changed

A tiny incremental release that simplifies deploying a testnet indexer.

Internal:

- storage: expose test postgres client and add migrations test by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/411
- testnet config: Add expected chain contexts by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/413

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.16...v0.0.17

## 0.0.16 (2023-05-09)

### What's Changed

This release brings close-to-final support for multiple backing nodes.

Feature: More robust support for archive nodes:

- config: allow connecting to a special node for some things by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/399
- nodeapi: add automatic runtime history provider by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/383
- config-related cleanup and opinions by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/390
- Cobalt core API: Support DebondingStartEscrowEvent by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/397
- api: remove nonexistent required latest_chain_id by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/393
- node config: Support testnet by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/412

Feature: Node response caching:

- analyzers, cache: Allow to shut down gracefully by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/392
- cleanup: enable offline cache-based runtimeClient by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/396
- cache analyzer sources by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/401
- file: don't crash on cleanup with query_on_cache_miss: no by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/402
- bugfix: KVStore (analyzer cache): Fix retrieval and error handling by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/409
- bugfix: nil dereference in FileApiLite::GetEpoch by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/408
- nodeapi: Store raw oasis-core in serialized form by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/382

Feature prep: Pre-work for supporting parallel block indexing:

- analyzer: remove the 100ms timeout before processing every block by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/385
- db: Disable/enable FK constraints on demand by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/400
- Andrew7234/epoch tracking by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/404

Bugfixes:

- bugfix: Typo in GetCommittees RPC method name by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/381
- analyzer: fix evm.Create result type cutting by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/403
- storage: unbreak eth address display by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/406
- Update discord link by @lukaw3d in https://github.com/oasisprotocol/oasis-indexer/pull/388

Other:

- For Sapphire support: EVM: download tokens at last processed round by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/380
- Andrew7234/api tests by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/360

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.15...v0.0.16

## 0.0.15 (2023-04-11)

### Breaking changes

There are breaking changes to the DB types, still not encoded as proper migrations. Either wipe the DB, or manually apply the following migrations:

```sql
alter table chain.blocks alter column metadata type jsonb;
alter table chain.events alter column body type jsonb;
alter table chain.entities alter column meta type jsonb;
alter table chain.commissions alter column schedule type jsonb;
alter table chain.runtime_transactions alter column body type jsonb;
alter table chain.runtime_events drop column evm_log_signature;
alter table chain.runtime_events alter column body type jsonb;
alter table chain.runtime_events add column evm_log_signature TEXT GENERATED ALWAYS AS (body->'topics'->>0) STORED;
CREATE INDEX ix_runtime_events_evm_log_signature ON chain.runtime_events(evm_log_signature);
```

There is also a breaking change to the config structure; consult #352

### Features

- Support for specifying multiple versions of archive nodes in the config: config: archive node configuration by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/352
- Support for non-ERC20 EVM events: runtime: store unrecognized evm log events by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/378
- Support for Cobalt blocks beyond a certain height: Cobalt: Support 2 versions of escrow events. Disable non-negativity check for escrow values. by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/370

### Fixes

- typo fix by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/376
- We no longer show non-EVM preimages as fake eth addresses: storage client: get entire address preimage by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/367
- Do not crash on malformed error messages in txs: bugfix: analyzer: Normalize error messages by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/373
- openapi: window_step_size: fix typo by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/377

### Internal

- db: Prefer JSONB column type over JSON by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/379

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.14...v0.0.15

## 0.0.14 (2023-04-04)

### Notice

A breaking (not fast forward) DB migration is introduced. We recommend a DB wipe.

### Fixes

- bugfix: db: Allow NULL tx body (for malformed txs) by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/374

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.13...v0.0.14

## 0.0.13 (2023-04-03)

### Notice

A breaking (not fast forward) DB migration is introduced. We recommend a DB wipe.

### Features

- analyzer/consensus: Full Cobalt support. Internal types for node responses. by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/356
- Support for pre-Damask runtime rounds by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/362

### Fixes

- bugfix: analyzer: evm events: Represent big int values as string, not float by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/359
- Fix typo in comments by @csillag in https://github.com/oasisprotocol/oasis-indexer/pull/364
- bugfix: Ignore entities with no address by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/366
- api: Fix runtime accounts query by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/372
- bugfix: Make entities.address NOT NULL by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/371
- add index for runtime rel tx queries by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/369

### Internal

- sql: reuse queries. update->upsert. by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/351
- config: add comment about sapphire start round by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/357
- analyzer/runtime: Refactor: Separate download, extract, sqlize steps by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/358
- add upserts to genesis analyzer; some cleanup by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/353
- statecheck: Do not use indexer-internal APIs for node access by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/361
- refactor: analyzer/runtime: Store a more-parsed tx in the db by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/368

### New Contributors

- @csillag made their first contribution in https://github.com/oasisprotocol/oasis-indexer/pull/364

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.12...v0.0.13

## 0.0.11 (2023-03-06)

### What's Changed

- Add `/runtime/accounts/{addr}`; add stats to runtime account endpoint by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/322
- db: consensus events: add index by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/338
- api: add gas_used and size to runtime transactions by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/337
- refactor: Initial support for Cobalt nodes by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/326
- stats: Implement Daily active accounts statistics by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/329
- EVM: prevent concurrent QueryBatch access by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/344

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.10...v0.0.11

## 0.0.10 (2023-02-27)

### What's Changed

- evm tokens: remove Emerald references by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/313
- EVM tokens: fix missing token type by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/316
- analyzer: capture non tx events by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/309
- api: add rel filter on emerald/transactions by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/314
- api: add eth_hash filter for runtime/transactions by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/315

- metrics: Ignore 4xx results by @abukosek in https://github.com/oasisprotocol/oasis-indexer/pull/317
- runtimes: add related transactions index by tx by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/320
- metrics: Use histogram for DB latencies by @abukosek in https://github.com/oasisprotocol/oasis-indexer/pull/321
- testing: emerald statecheck by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/319
- api: Add runtime transaction sender and receiver Eth address by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/310
- minor: include codegen files in docker image by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/327
- api: include pagination total by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/324
- cosmetic changes by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/330
- log: automatically include caller and timestamp by @ptrus in https://github.com/oasisprotocol/oasis-indexer/pull/328
- db: runtime_events table: add indexes by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/333

### New Contributors

- @ptrus made their first contribution in https://github.com/oasisprotocol/oasis-indexer/pull/328

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.9...v0.0.10

## 0.0.9 (2023-02-10)

### What's Changed

- codegen helper: Use POSIX sed (OS X compatibility) by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/308
- bump oasis-sdk by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/307
- runtime: remove some Emerald references by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/306
- EVM: remove chain ID check by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/311
- metrics: Normalize endpoints, use histogram for latencies by @abukosek in https://github.com/oasisprotocol/oasis-indexer/pull/312

### New Contributors

- @abukosek made their first contribution in https://github.com/oasisprotocol/oasis-indexer/pull/312

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.8...v0.0.9

## 0.0.8 (2023-02-03)

### What's Changed

Internal cleanup/refactoring:

- db client: Simplify, clean up by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/297
- Generalize emerald tables to multiple runtimes by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/296
- analyzer: Refactor configs by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/299
- openapi: Generic runtime URLs by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/300
- Emerald: tokens analyzer by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/284
- storage: source storage timeout (fixes Emerald analyzer freezing randomly) by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/276

Testing improvements:

- Basic manual e2e regression test suite by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/285
- genesis-test: Create DB snapshot using full tx isolation by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/298

API extensions and changes:

- Andrew7234/emerald events by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/301
  - Api spec + db changes for indexing emerald events by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/295
- /runtime/transactions: Populate the timestamp by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/304
- openapi: name the root path as getStatus by @lukaw3d in https://github.com/oasisprotocol/oasis-indexer/pull/302
- api: Implement missing/empty fields for MVP by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/289
- Track native balances in runtimes. Per-denomination events from `accounts` module. by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/288
- api: Support CORS by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/290
- Implement /emerald/stats/tx_volume by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/294

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.7...v0.0.8

## 0.0.7 (2023-01-19)

### What's Changed

Same as 0.0.6, but with fixed github release pipeline.

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.6...v0.0.7

## 0.0.6 (2023-01-19)

### What's Changed

New features:

- Add account related txs and events by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/277
- Emerald: token queries, handle deterministic errors by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/265
- Implement `/emerald/transactions/{tx_hash}` by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/286

Bugfixes: Improved handling of bigints and some other types

- bugfix: Convert bigint params to string before passing to DB by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/272
- bugfix: openapi, Go: Various type problems by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/283
- [api] check for divide-by-zero by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/216

Improved/guaranteed type compliance with openapi spec

- refactor: Autogenerate Go types from openapi by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/268
- Autogenerate HTTP server from openapi spec by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/280
- api: make epoch end height optional by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/278

Internal cleanup:

- api: extract replyJSON method by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/275
- db: Add index for consensus tx hash by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/282

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.5...v0.0.6

## 0.0.5 (2023-01-05)

### What's Changed

This quickfix release includes two PRs (269,271) that aim to prevent the indexer from choking on well-formed blocks.

- evm: require ERC-20 totalSupply by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/269
- Emerald: fix legacy EVM and ERC capitalization by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/270
- bugfix: Process fee accumulator disbursements last by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/271

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v.0.0.4...v0.0.5

## 0.0.4 (2023-01-03)

### What's Changed

- Emerald: list blocks api by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/190
- api: specify inclusive min/max filters by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/200
- nit: Rename poorly named func by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/204
- Update e2e tests documentation by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/191
- Change delegations rounding->floor by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/210
- Emerald: store transaction result by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/201
- cmd: flag for wiping the db on startup by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/214
- Strenghten sql constraints by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/217
- bugfix: stringify big.Int and uint64 before sending to pgx by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/221
- Small readability, logging, debugging fixes by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/219
- Emerald: list transactions api by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/194
- Emerald: track tokens by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/213
- Emerald: add index for token holders count by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/215
- Emerald: add tokens api by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/218
- bump oasis-core to 2202.3 by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/226
- Emerald: add tokens api spec by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/224
- genesis_test: Numerous improvements by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/227
- Update concurrency of process block goroutines by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/228
- Allow partial configs, optionally run just API or just analyzers by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/230
- Andrew7234/genesis test by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/212
- bugfix: genesis_test: Ignore expired nodes in registry by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/231
- Update fkey constraint by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/234
- Update db types by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/237
- docker: per-user dev tags by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/239
- openAPI: simplify stats stack, publish html version by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/241
- db: fix permissions, search paths by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/244
- Andrew7234/bigint by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/236
- consensus: refactor: fetch all round data at once, then process by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/243
- Do not fetch ChainContext when known by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/233
- consensus: Do not track runtime (un)suspensions by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/232
- db: Rename fields txn*\* -> tx*\* by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/252
- openapi: Consensus events endpoint by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/246
- openapi: Add assorted fields required by MVP by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/247
- event enums: Rename to conform with oasis-core by @mitjat in https://github.com/oasisprotocol/oasis-indexer/pull/253
- [db] minor type fix by @Andrew7234 in https://github.com/oasisprotocol/oasis-indexer/pull/254
- Emerald: query token name etc by @pro-wh in https://github.com/oasisprotocol/oasis-indexer/pull/229
- api: Show large numbers as strings by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/251
- analyzer: Index all services backends' events by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/188
- Add account on new allowance owner by @aefhm in https://github.com/oasisprotocol/oasis-indexer/pull/266

**Full Changelog**: https://github.com/oasisprotocol/oasis-indexer/compare/v0.0.3...v.0.0.4

## 0.0.3 (2022-10-19)

### Bug fixes

- 07a16fb15af0c441a62de56b1681f41cb226e44b: bugfix: pipe ChainID into consensus analyzer (@mitjat)
- d7a139955185fc57e3a1480fd28ed8616bbabe20: bugfix: typo in balance update (@mitjat)
- ee88f5688ff2805883522a8ccd33a63ac07873f4: generator: bugfix: do not generate INSERT stmt for 0 rows (@mitjat)

### Other changes

- 0553ab41e1136cbf26ead9fd3d01a7cc237bc4d5: Add Docker workflow comment (@aefhm)
- 2d2f06295884c150db21df4e3a5c763d6a174571: Add TODO for parsing migrations from genesis file (@aefhm)
- b2716be65bd8a8d542f0a10fb266670dd6edfa88: Add e2e tests (@aefhm)
- 79f26874143920ec7e7d5ac6b9e61e88effe4f49: Add mitjat to code owners (@aefhm)
- b9db70481dfa76132929d3b90647e962c8223a6b: Add net-runner Dockerfile (@aefhm)
- c3175fca7e673f0c6c8c0873cefe6d31b31b0e9c: Add tracking of genesis processing (@mitjat)
- cde97b80f0ce226af150175d779a05cb077e1ea6: Bump golangci-lint to 1.49.0 (@aefhm)
- d07323ae10800bc4279b362d03e80d4928d070fe: Configure server ChainID (@aefhm)
- 696e0bf1bf648d88d2cb8cd611ef7005c0b3b2ab: Consensus, emerald: reduce logspam when caught up with blockchain (@mitjat)
- b8dfe75479d9c88b47658d47908c0243b8511efc: Emerald: 0.01 effort paste from nexus (@pro-wh)
- b6be0655374701c4c77fcb45211e42a66aea96f3: Emerald: Rename height->round, use timestamp type (@mitjat)
- 540f44197d78f970d0b4894502c8dd7b8676e982: Emerald: add comment heading (@pro-wh)
- 5b3f4e481101137e1cfe38d138c9e8f55299358a: Emerald: add missing error wrap (@pro-wh)
- ee53231de0548d5ffa858fdc81b774cb45dca6fd: Emerald: add new inserts to QueryFactory (@pro-wh)
- 26222b385ef1c45c71208c183bbe6142a9b66821: Emerald: add note about block gas math (@pro-wh)
- e7fa5bd5b7c3119a39233778f657a674be66750f: Emerald: add signer_index explanation (@pro-wh)
- b1fc6b63e8996e7dff814db567ab5535c08fe4ab: Emerald: adjust index methods, add block hash index (@pro-wh)
- 9700ca2e69f6e40c3c1194bdb60f4bbfda0f2152: Emerald: combine block and transaction inserts (@pro-wh)
- f03af901532c3e047434d8d06304904884fe4b89: Emerald: don't authenticate transactions (@pro-wh)
- 092872cd29152f4f9b0b337e88b8ac1bea9dc6d9: Emerald: eliminate uint64 -> int64 conversions (@pro-wh)
- a73be9573b755a97cd906dd3e6004ac949f7aeea: Emerald: explain address_preimages (@pro-wh)
- 81f36380bea71fba11344814182954083b7b3785: Emerald: improve tx decoding error log (@pro-wh)
- 9cf4497a7caeeabe7f239a48e0c9c2025d0dc798: Emerald: merging databases (@pro-wh)
- f6a7b2bba4c40cbf9ab3bd84425b6d8c9f218b95: Emerald: printf -> log (@pro-wh)
- 68d78f28ea33f4a13df1c2d8261b4894134b64fd: Emerald: put test data in structs (@pro-wh)
- 6e39f28166496c2f2a86f0ae9cac1434cf0920b9: Emerald: rename loop var in long loop (@pro-wh)
- 6a77efcecb16452685373b88b061cafaae239a9b: Emerald: store raw tx (@pro-wh)
- 6c4b311f4043f5bdf296d4aa041abe072738b125: Emerald: store raw tx as bytea (@pro-wh)
- 209444cf75a9f4ca38d8314feec4944402518edd: Emerald: update to storage.QueryBatch (@pro-wh)
- 7c07ddfc4cd278f646cc17a05487d8865317edca: Emerald: use bytea for address_preimages.address_data (@pro-wh)
- 42821c7f1fa41cb5054e1f356a38688ebe0a2c75: Emerald: use hex for address_preimages.address_data (@pro-wh)
- cc0126c044dd5c3d42461ec551577f585ae064d1: Extract test helper (@aefhm)
- 34b78105f7fca1f450c6239a436975690a86656c: Fix Docker dev configuration (@aefhm)
- 8b23cc387721dcfe7cc55a03e5f3e96f29631af5: Fix epoch bug (@aefhm)
- e731069b52a1b2b5071f5b6af2e538f76526bc6a: Fix missing Epoch in StakingData (@aefhm)
- 2f027842a436ba6d104d61cb0866816789a71335: Fix util helper (@aefhm)
- 267a554b4206e10e77aaf2de745e0aead13fc9bd: Ignore testnet dir (@aefhm)
- 51f12ce51cdbcb115bb3f10a382872937430a21f: Improve Docker dev (@aefhm)
- dea7d09f4572938240475f0345a547856d5cbf63: Improve comment (@aefhm)
- dcd8a9a056d1612caf60ffa410afc1d8ff91c056: Improve e2e test error assertion (@aefhm)
- 3676c1fde64fd22ec7e974404e9bac40bbfa8a5b: Improve latest Indexer status documentation (@aefhm)
- d6f78d9a2df853c5b9712be02b50c145e0c11b94: Improve net-runner Dockerfile (@aefhm)
- 117ba8a93c636f0690d5e19da62e4191c0441197: Improve testing logging (@aefhm)
- e793708a95fa18c84fba259a94e4ff7efed74637: Increase e2e wait timeout (@aefhm)
- e3cdced70fa241bdf9dbf4b37666b76bafe6a8aa: Makefile, READMEs: small fixes and updates (@mitjat)
- e48151cf5f977937370c7ec04b28eb065ab1345a: Makefile: more robust postgres target (@mitjat)
- 21ac5ae02c43c86e149088b70efbada6cdd786be: Makefile: psql target (@mitjat)
- b70e97f8d5f25cf1ad306da031a357cb88e355d0: Migration for our schema (@pro-wh)
- 5efaf8d9e01d2ddd39166cf2ffe31ff7a1d1e254: Move generator to oasis package (@mitjat)
- 14cbf7d73183bab1cdaed79620147efd5c6936cb: Patch gosec linting (@aefhm)
- 282084a8fd2cb742d600a5861a6a1024c7745434: Patch test config (@aefhm)
- b2c91123b9a1d2a55843593326cbcf7cdb510249: Remove support for CockroachDB (@mitjat)
- f60f4e08b4e1d3be0fe97879b09d42aafd2e642c: Send generated sql to DB (@mitjat)
- 4f402689ac37f7a685769e297779c579e3604696: Separated lint (@pro-wh)
- 67be1879148ac4be43c4ba135b0a73c593a8faec: Simplify goroutine invocations (@mitjat)
- 47979a5a499ae282cd5fc57b192ebbd0b734959f: Test with test chain_id (@aefhm)
- 8ff35c4c5a9b3c04fd878f282b42c3e328b8b049: Update e2e test Docker compose path (@aefhm)
- a5d3b7bc1cbbc9edfda9dd840a54b08326de33c9: Update http server with timeouts (@aefhm)
- 08ed05047e5a4b7ce3726a4d0d2b35f7ace974ca: Update ignore files (@aefhm)
- 5bf0b3e710538c9958749ba41e8089ec964bd4e3: Update latest_update time display (@aefhm)
- e42ab1878774949757e11fefe66a57f05591fcfb: Update to golang 1.18 (@aefhm)
- b2d0cb7210e00a35c7de96906a86a9d58e10654d: Use Postgres version 13.7 in testing and dev (@aefhm)
- aa69e8e6dab17654f57bd042cbbe204238190fc6: [api] add in-memory cache (@Andrew7234)
- 7cb5eaa9c28f00d9ee3a9f711d583b23d97cc429: [db] Log failed queries (@mitjat)
- d19ecd960b23ec47c3835445d1fc71e2874773bc: [db] Pipe postgres logs into our logging system (@mitjat)
- 4396501ea2c63bcbede347b1cb9513de79f00954: add warren to codeowners (@Andrew7234)
- 6b3be0d3e0b8443172b53ec8c8b0a70ede0a5358: additional docs/comments from review (@mitjat)
- cffd11322e964f9561b597d3f26224db72421452: address comments (@Andrew7234)
- dad59259c542f61761a91f4a749749e6827b6a60: aggregate stats: Increase interval, timeout (@mitjat)
- 4fb16fd36262c1f41d52ad1227b8667f6f52fcef: analyzer, config: expand ChainContext description (@pro-wh)
- 4642bd8ea0c241e20e2993120e332538044e57b8: analyzer: change RuntimeConfig fields (@pro-wh)
- c8dc908072b6d41fd543daa172b171cf4742dc27: cache block and tx endpoints (@Andrew7234)
- dde4e558f5a827896135cf6b642940d5b796eb87: cache genesisHeight in source client (@Andrew7234)
- 090ee16b4e4cc7abb7c8994f6157938382592257: cosmetic review fixes (@mitjat)
- 7852e9112522b2954d3f266531886c1a0649b661: db: Merge migrations (@mitjat)
- e61b97c83c6644e39585f44db06a206de05ac42c: db: Remove extra_data fields (@mitjat)
- 23fd45c7b3a0dde35ef1001463c17dd6554fd711: e2e: do not use a separate set of migrations (@mitjat)
- d2f76e629d004c374b9e17354a7910785bec1a15: fix address/pubkey validation; /transactions/tx spec; dedup storage.ContextKey (@Andrew7234)
- 38a6fbea3cc72804b96322969debf8f6919b7614: generator: Change verbiage: 'migration generator'->'genesis processor' (@mitjat)
- 5f7601aaf628d7a04f885d1b9f488b90fff1d6d0: generator: log if closing output fails (@pro-wh)
- 7f50d314c5c268f303e4de918f1f35d2ea0f9730: generator: produce deterministic output (@mitjat)
- a3b46202121a91973d99ff00236e98d722b79bee: generator: refactor into returning list of queries (@mitjat)
- c7aadad009105ca6be113baa3ebc309f55c5340e: generator: remove cmdline interface (@mitjat)
- 3207232d3a192ff23c4d8f2f9f7f293ba83b56aa: go mod tidy (@pro-wh)
- e02ecfa2419b3834653803207a456c9db7d2a6ee: infra: update github actions (@mitjat)
- d8d94c3367b125432f85969271848a0939d2f1c9: lint (@Andrew7234)
- 3bb52f85c9674c270d26f13b2b712ddf382c9d7b: lint (@mitjat)
- 9ff6557953747e9d110729832a713d1a3af3d22a: lint (@mitjat)
- 850bf3637d7c18b488392f407e65179b84b01f83: lint Id -> ID (@Andrew7234)
- add70f2ec440362750ecc10dede2ad922904ad77: linter (@Andrew7234)
- bd4d6041f6a2785a65dc90023c98affdda27f062: more/improved logging (@mitjat)
- 70048d039e23eec01c0af9f224dd5dbd3f2ca9f6: post-rebase fixes (@Andrew7234)
- 20ca2046e1af6ee7c5fda9006871b3116ccd9776: postgres: tests: Use SkipIfShort utility func (@mitjat)
- 0fafa772cca76a26f342346b62d35a485d897508: refactor sql out of api package (@Andrew7234)
- eb549f96118f834b3f150354f5db06dfc401e0d9: remove \*height from openapi spec (@Andrew7234)
- 57f2a6d536bef3b3b7103801f946831e27fc1560: revert timestamp->produced_at rename (@mitjat)
- 7e4dcb7fc83a73c05d112b709a59d07faf758a6c: simplify ConsensusClient impl (@mitjat)
- fbce3813b284ce18207896b23acb30b8b941b2e7: storage: log if closing batch fails (@pro-wh)
- 311bfabe63e6fca96a342cd43f5f584a225b2594: strip "main" from analyzer names (@mitjat)
- b2fffe3f73fbf3082815cbc4efd78399fd78d25d: update docs and name (@mitjat)

**Full Changelog**: https://github.com/oasislabs/oasis-indexer/compare/v0.0.2...v0.0.3

### Running the indexer

https://github.com/oasislabs/oasis-indexer/blob/v0.0.3/README.md#docker-development

## 0.0.2 (2022-09-01)

### Other changes

- f2dca716e101fc41e7f52414b8c942757b22beca: .github: Add Andrew7234 to CODEOWNERS (@ennsharma)
- 8d178ee97555e10c2c4ec3af5c443c2574c72f09: Account for network upgrade debonding (@aefhm)
- b359585a7abd86a813531721e009f2cec72c71cf: Add Epoch to StakingData (@aefhm)
- 15615351785c52ea3f40c71cde632a3cb8464585: Add TODO for inaccurate column (@aefhm)
- dcf430da4b7635030ffba6ed70ceb7155afa5ee1: Add amount of delegated shares (@aefhm)
- 33dd0bddee2bdf9cddd94284eb335089654b36b0: Add analyzer for commission schedule (@aefhm)
- df21fae88913c548275882e50e3b80dc018d074a: Add back pagination to 1000 Validators (@aefhm)
- f034b5952d75ce47cc5e5c558ea9ff2bdca8d9ee: Add blocks sanity check (@ennsharma)
- f9195e4480258459945edc9e7747d402caf9c8c3: Add delegations and debonding balance to Account response (@aefhm)
- ebe1bf42815e0cff1c1b7a171dab4ef4be7f5b7a: Add delegations and debonding delegations (@aefhm)
- 7b016d4b0ec2a17a4dd5f153c99b3c4c345e2b4f: Add e2e guards (@ennsharma)
- 8ab3f46f28e09fd94c1393513d989a456b3946bf: Add epoch analyzer (@aefhm)
- b0458e650d4a5bf35ccb6dc56bd3910701a60797: Add governance checkpoint test (@ennsharma)
- e074e60fd3e060f69497ac5218e2c184886e5c07: Add metadata registry entry analyzer (@aefhm)
- 8031a9f112580b82c16543b5fdf1b24dbd95b9b1: Add migration for reserved addresses (@aefhm)
- c5c013ad20f623410738dfdda89c06887a73ceef: Add migrations file validation (@ennsharma)
- 7cb6ad2f6c1599618ec0c458ea2c518e720a6bfc: Add missing Height param to client returned data (@aefhm)
- 28e439a6290816821cb2a7e727801c7634eb65ee: Add ordering of Validators by voting_power (@aefhm)
- 885613183bf6bd4d44355ce5fbba7e085b2c28db: Add reserved addresses account generation (@aefhm)
- 266eb28b0f17c8539e252db20a129c5f7f74dcca: Add round processing (@ennsharma)
- a2a84acf720fb7983217854b152122ce69ae4a39: Add runtime context to tx verification (@ennsharma)
- c0653864d402177c1a26ca5d045fa79cc8faa9be: Add tx sender index (@ennsharma)
- 0527b5a7850f3cafd4d054af2ce2c7080fdc4721: Add up migration for debonding delegations id column (@aefhm)
- 96dd3a8c3d9673d91a8c2261ae7508b55fc4ef70: Add validators (@aefhm)
- ba9ad439d3ac6c140389e87c0539db38b801d68a: Address review (@ennsharma)
- 73b7067c3e4c01d3862359ce691ef586303aa684: Bump go-chi/chi to v5 (@aefhm)
- ded165abbc6d4f4ec896131fa3029bf39c2e909a: Checkpoint for the day (@ennsharma)
- de373b12a16f92d37e9a3a166a719d33a072c9f1: Create shim for runtime storage interface (@ennsharma)
- a079fa256ed060241b90e2a217406888938ec1f5: Debonding delegations totaled by end epoch (@aefhm)
- dd3161491a6b3a4d2818ccea3f6f85877136464b: Fix API (@aefhm)
- 5529836307b14674d0ace837642be00b4a82d0bc: Fix GetValidators return key (@aefhm)
- b5a77c7cd7b729417238e0ff9f90c038cc7b86c6: Fix ListEpochs endpoint (@aefhm)
- e1481b26e273ea1a4097708995eb86bc0b077945: Fix Validator node selector query (@aefhm)
- 83501c1ba27152ad5f462164482fa56e56718817: Fix analyzer (@aefhm)
- ca9cb879656b7b1b10269498d34c1ef76ebeba99: Fix client typo (@aefhm)
- fd5dfbd6afb0a15ac25ec5d3a670bc5c32f2902d: Fix commission analyzer (@aefhm)
- 2e81871e005c163527abf113f12013d28603df9d: Fix debonding delegations (@aefhm)
- 9ff713c2728eda05644037bec6cd9be6efeb34d3: Fix deletion of debonding delegations on reclaim (@aefhm)
- d02a1bed03f04c5f7d350bebad6fc5197aa0b483: Fix deletion query (@aefhm)
- 0093d0b313ded70347097ea4171824f4b4e1439a: Fix doc building command (@aefhm)
- e744df3168f74021914e73f50f589c21bd90bfe6: Fix duplicate debonding delegations (@aefhm)
- d811a9063256bc63963fa97794cfcf72cc8d4846: Fix empty responses in lists (@aefhm)
- 8878c1cefe05ee854578bd1f9272acf77f3aec3c: Fix epoch and bound bounds (@aefhm)
- 445b3f2e037bd6875645628ed737c572c9aa9346: Fix escrow debond calculation (@aefhm)
- 3806c4b0c7685918a149660ef6e06a341f3b3f27: Fix escrow on slash (@aefhm)
- 1b47a886e19bf528ed5410f299ceda54a48cc2bc: Fix large number overflow (@aefhm)
- 6ebf86672c7d790336e3cbd929788f80177dc142: Fix lint (@ennsharma)
- c3b6db75e55f0975bbb0f24dcc750d0ab5471476: Fix lint (@ennsharma)
- 1e47bb9a783dafe8598ac2a80eb6db854d953732: Fix negative delegation shares (@aefhm)
- 2e5abe49f566fbaa8eee2e1299e209b30ce7d55e: Fix omission of Validators without commission schedules (@aefhm)
- c6bdaa678db0009f534febb627a5236683e1bd97: Fix staking tests (@ennsharma)
- fe70a433828346ece1de6b3cd4933f1db919c0a5: Fix whitespace (@aefhm)
- b61888f6e5d044357ac9031f2297acd502405692: Improve API documentation specificity (@aefhm)
- a76a0bd3a830767dd72e7cb7e6b890a05c8a2673: Improve SQL style (@aefhm)
- f515752c2c5e213aa30f35da368ce502bc94d421: Improve error checking (@aefhm)
- e8dc1e40f50a4461e0415aec35386b937a96a668: Improve style (@aefhm)
- 367f2ab128cd583062cef82f88758351481c2b78: Initialize framework for aggregate statistics (@ennsharma)
- 9c74bd25e2b65cfe8df329234eb2fc4e50164ebc: Initialize shim for emerald analyzer (@ennsharma)
- 10925cd28a91ec063f1ac6a15aa76b46dd602b31: Initialize source storage for runtimes (@ennsharma)
- 2089889a574ea34ba1a3c05f9ff56e6606349c4a: Reference default pagination (@aefhm)
- 47ef9894ec12fa77b60f955b795c64442d701fed: Remove default pagination for Validators (@aefhm)
- c435859310d375eec7958926af0862e1d52e1da3: Remove duplicates (@aefhm)
- d94b3620ddf290aed235d6210dde489f02706337: Remove reclaimed debonding delegations (@aefhm)
- 4a407de04e10249695e6fddac9b9fd192441f6c1: Remove total field from Account (@aefhm)
- 66b4f4e0650092e9f8f7b420b4d3af0316f604bf: Rename go module (@ennsharma)
- 3e7e601f4aa73428020c5f6a2583da4c3c92607b: Rename migrations to address conflict (@aefhm)
- eadb1f96c56ccda143607dcaa9cab13eaf429ba6: Rename sender index migration correctly (@ennsharma)
- 36012312318f2cd84111f015cd9607c6f0c4d812: Update API spec (@aefhm)
- 5bb63a043b3126136cfbcc1f2184d2b17ebc0930: Update Go dependencies (@aefhm)
- 6d71a6ff73bd084a0f3a40a0629f6400ceb7377e: Update Go packages (@aefhm)
- 4b3bd190ec3604ca74c69b4a30076a6c70b90af1: Update Oasis dependencies (@aefhm)
- 28c51bf9122054735a2669d905cbfe6945c753dc: Update README local instructions (@aefhm)
- 24b22f01e0520f7da8aea7986ec97f7eefe24d93: Update go.mod file (@aefhm)
- 487fae8984e9d2287e15f2b1497088d53bdeba7b: Update v1 API spec (@aefhm)
- eaf3515506096a5766e2d873f54b6f73b7dc030e: Upgrade github.com/spf13/cobra (@aefhm)
- 528e729d60256a9ff0de6e790235b6e6a2d6fe9a: Use network debonding amount estimate (@aefhm)
- e2f6674c81bcafadc78a2b32edf478bb552228d2: add additional ci test guards and allowances test (@ennsharma)
- 3262c6a938fa2fc31a972c38ca3c8085e24a7e00: add governance test (@ennsharma)
- c94e000cd44a9b5f78d8f0cc50af6783c11ad703: add some additional logging (@ennsharma)
- 58ec675ecc4b7ed8d94a633cac9ecea7ed2157d9: address comment (@ennsharma)
- 21f02b0448f88cfd63df19fef3735b9f18ada925: address comments (@ennsharma)
- f54699d8c9ef027185b0f426ff932610b47bce81: address comments and lint (@ennsharma)
- 6abd7d26a3d3fbce07827ca2e8c3c5fbaf3abff6: address lint (@ennsharma)
- 44f1bd94311f421ead2e580672c9e0060780ba5b: api: Minor transaction endpoint fixes (@ennsharma)
- 0547171e6c6391d156c06b83624678bd870fc19a: apply lint (@ennsharma)
- dd74a6ef8e154bf31691d41a7df5fbe95bde924e: apply query factory suggestion (@ennsharma)
- aa0b7e24a21a2776bac3d0bb62b67dd77c56647d: bugfix (@ennsharma)
- 8c7341ae9b54635899e8330cdef794a5d7c947ea: do with just healthcheck test (@ennsharma)
- 474842143fb81fa8fde07c99c8d2b3c0eef14f82: finish modules processors (@ennsharma)
- 11d92e971e83d2ae5b2bd7a3575b71a9002f6170: fix 104 (@ennsharma)
- bdc020f15d052bb190031bcd11ed5c5f20b23991: fix bugs (@ennsharma)
- 542317d361b44500cd24043ce0a425bba55051f2: fix env var name (@ennsharma)
- 23bc9cc5deb8932ffa94e38c004beaf2eddd2fac: fix metrics (@ennsharma)
- 2cf57a726b8860df8048057a29deb485e9d9af13: fix more bugs (@ennsharma)
- d173e25dc8006e1a25ebd0cdce1aba6a00a1cbc0: fix openapi spec (@ennsharma)
- c6169f5c94663f8e4f756dc6b345e27096967cce: fix some bugs and add registry tests (@ennsharma)
- 59bd3e0375c8a600013b48d69a79b90cfa12f456: fix test (@ennsharma)
- f175f41271f0f463be0b97170535af7c1fb3b26a: fix timestamptz (@ennsharma)
- 286bc0f1420dbc6a6e789a34b8ff58116625f65b: ignore lint on single case switch (@ennsharma)
- 417070e9059dc92d0cc4c10cfc3a9e14716ba5ca: lint (@ennsharma)
- a1021c51b958d4e14d7690b8860e9cc308a2d09c: make tests more ergonomic and interpretable (@ennsharma)
- 86da95fd972c0206b2af046c387ca92562a512e9: merge (@ennsharma)
- fc2847115f2cf29a4d4057d7c7f242e5b274fbb8: merge (@ennsharma)
- 699a599d14f8566cc27575a8c90f816721093e81: merge branch (@ennsharma)
- 0fb073f2ccf7bad2f4f827af4ff5159b38a52489: minor nits (@ennsharma)
- cf74274b6e3116b69f5b4199cc13fb1d0bf2a36c: nit (@ennsharma)
- 7691b261dfcbc81bd6df8587233743d5a1b645dd: paginate more goodly (@ennsharma)
- 57fd08f0c35807b748f2e5a22253715874117700: remove pagination.Order and use defaults instead (@ennsharma)
- 3c4fcb0fe6706e6654f96fb6fb742af0cbc60118: remove spurious log (@ennsharma)
- db120df09a96b903dd9a123d493c81a2416ca3c8: resolve 101 (@ennsharma)
- 33db98a462636778def6c9aa0f7f144e5a5531cf: resolve 106 (@ennsharma)
- b918b24773af5b820ac21ead88eb1ee66c3af9d7: resolve 107 (@ennsharma)
- db39d20f069b933d387be3ef888452c20f01d70f: resolve 108 (@ennsharma)
- 2b22f870d2bb4e1b709facbe8f9dc39c45f536c9: resolve 109 (@ennsharma)
- b0c74e723f5d7ddc942f4559121eb6382f922cb4: resolve 61 (@ennsharma)
- d95922ad308e3f8abee5ce9d0e9cb5138527e601: resolve comments (@ennsharma)
- 75e7099a2b465832f4e7af22b20e2cc351005633: resolve conflicts (@ennsharma)
- c1bf5643d55a76db20f3c03e85c6e882319b7279: storage: Add oasis client test (@ennsharma)
- ec6fe05ac22b45354381aba7d7833545ca46b545: tests: Add registry and staking basic e2es (@ennsharma)
- e555dd86dda7121076beb7937479a6605a8a85eb: tests: Add staking genesis test with checkpointing (@ennsharma)
- f11a1b7df73d73c9bb988f272c2d13402350f19e: tests: Fix registry entities validation (@ennsharma)
- ff1cad1032b8c8e1afa15da7a87355cf3e1a7769: tests: Fix runtime genesis test (@ennsharma)
- e8c22fdf21d2e754c600742c62af9c0421f7703b: tests: Initialize e2e tests for blocks, transactions (@ennsharma)
- 6a91a658ef10357bd4543542899ddc26c8cdc3c1: use main branch of oasis-sdk and dynamically set runtime id (@ennsharma)

**Full Changelog**: https://github.com/oasislabs/oasis-indexer/compare/v0.0.1...v0.0.2

### Running the indexer

https://github.com/oasislabs/oasis-indexer/blob/v0.0.2/README.md#docker-development

## 0.0.1 (2022-06-14)

### New Features

- 53ba92013ff486c3e988fe73b285725ec1bfede7: feat: Deprecate (most) flags in favor of config file (#63) (@ennsharma)
- 1b74ddf85c6846913bb5d72d3c3bfd981ca886b0: feat: document release process and improve code hygiene (#58) (@ennsharma)

### Bug fixes

- ed5e39221fdb2ebfaa948ff167cd8748a5cdafba: fix: Implement log test suite (#70) (@ennsharma)
- ff1f75205ed3cb650b41187c3400e524f1d605b9: fix: Implement target storage test suite (#68) (@ennsharma)
- 3acb03b70ea8ff8fa7a017e1d063d026b91a256f: fix: Switch LICENSE to Apache2 (#72) (@ennsharma)

### Other changes

- 7b0eb1e8ec15ed4184be2b325d63eb1a6073e3b6: .github: Build docker image on commits to `main` (#38) (@ennsharma)
- b2075b5d7fd2b79193a059e5fb08ef0674037a5e: API Layer (#28) (@ennsharma)
- 4ba2540cd494e19fcd8745ed35c47112ed771be3: Add cockroach client (@ennsharma)
- 94fe3d2f6b62f7acd3b9dd94952d42a7b9331dce: Add connection pool boilerplate (@ennsharma)
- c60ea64baad2cd1eaf1ac89b233eced7e0a2c6bc: Add generator subcommand (@ennsharma)
- ac3fafcb9f559b954bcc4d80e7a1d5cb2750bfd2: Add gitignore file (@aefhm)
- fb83554609ac9d150cb6cea7c11cdf5a262a8a5e: Add indexer Dockerfile (@ennsharma)
- 749f8f8475148cf3f0ed7fe60044788d430cba5a: Add indexer dockerfile (@ennsharma)
- 457aa2868a49f258a99ab82aaa8098bbc5e25c0e: Add indexer dockerfile (@ennsharma)
- abdcaaf138cdcf71ef4fed9c1a3ef058dbbeaec8: Add initial migrations for target storage (@ennsharma)
- 1d09b3f48c96f372abee771aaa1cb25707bdf3c4: Add lint check (#45) (@ennsharma)
- 5c9d4a3f1baa6afc36eac3dc9e85c70b27b1e311: Add more boilerplate, misc stuff (@Andrew7234)
- e189fe88185e2e4943b1aaa969222d5d8e44b002: Add more local instructions (@aefhm)
- 54c3599db24bed28a160a2fc2c9bb1d91dce2894: Add oasis-2 chain migration (@ennsharma)
- d758c15ad3d5593e472c3fa1f480b434bb2daaa6: Add oasis-node client (@aefhm)
- 8843d3e317786a12246693465efb5c620b463e9f: Add oasis-node client (@aefhm)
- 4b08e0d53472ca82fdeaa98a8eb2ba767da17066: Add prometheus pull service (@Andrew7234)
- 47eccc6de97d3b0a05145a546c27d0df144f0b59: Add registry, staking, schedule, and governance analyzers (#48) (@aefhm)
- 881cb22ebb313137ad35ce25ab0991c66851ec56: Add skeleton code (@ennsharma)
- 4e1c9262a998ef7cd14a781bd4333ccfcddc3df8: Add some minor comments (@ennsharma)
- 6c3102cd0573833dbe48763c81e170bf1ac78336: Add some template default metrics for db and requests (@Andrew7234)
- fa9bb6216a0a19ea0b57e6f7103bc0eb2cda6dda: Add target storage interface (@ennsharma)
- 0a3c2121aa8f69e9052507fee6de9b7364d03d71: Address review comments (@ennsharma)
- add872cd96c1541ac9c9d73ef908d20a68223b24: Bump Oasis SDK and core to 22 post Damask (@aefhm)
- ab2be3b5f0355c532f99dbfe239c911efef1c7dd: Bump config chaincontext post Damask (@aefhm)
- fd82649585f153182c52f1cd2fcbaed45fb07f4b: Bump workable go.sum (@aefhm)
- ffaec26db8ba88426786429771f01fe47766215b: Connect to CockroachDB locally in Docker (#34) (@aefhm)
- 8044067712003246b2a2f6ed12374a3ec19d38b4: Enable Analyzer Auto-Migration (#41) (@aefhm)
- 1fe4c6aeba2a923f07a7ce4e6972d72e69edd050: Enable generating migrations from file (@ennsharma)
- e08cadf3d8c02cb171beba1e2f2191d639319b2b: Fill in storage API (@ennsharma)
- 2a58a28d06e835b6c51210e0e9f73bdf7231f9ac: Fix (@ennsharma)
- af9a5894bf834d6353a7b37de9462caa54f52585: Fix config typo (@aefhm)
- fd32129933bc774b1b6637ae39fcb89096a25856: Fix typo in RPC endpoint (#69) (@aefhm)
- 61aabb88704bf4e655e98c6a411fb5c6215f6961: Flatten go/ dir (#37) (@ennsharma)
- f30b2f5b21a0d4132070d506a0ed4fc15d85d4c6: Initial cmd shim (@ennsharma)
- 2ced1dc306135a3b5697122eb726cff5fe7e9ce6: Initialize storage interface (@ennsharma)
- 87ea6d5743c32614e38410ece31dfdba5458ec2f: Initialize structured logging package (@ennsharma)
- de72c57c26455c3efea70cf2ca9fb3758115e1ef: Integrate common logging (@ennsharma)
- aa46bb2e2c7ba1a839f46d88acb31aa779fb4cff: Local Development via Docker (#30) (@aefhm)
- dda50d92f4a135a6530d0f36972b3afa21ebc6d1: Minor local dev fix (#57) (@aefhm)
- 608db9fa4f0487914e441a716ea1b3104091ef0a: Move init into command (@ennsharma)
- 6774df826497a8a2f996771197100d24f4c1fdce: Remove processor module (@ennsharma)
- 66dc527b1a284ff19e8ecce0f0c160d1fbe23599: Update README.md (@ennsharma)
- 79725aecf7008f2766f60f78f942689a7bfff4a7: Update config and readme (@ennsharma)
- b21b6b5be87e4a978056d9dff759378012258547: Use Postgres again (#47) (@aefhm)
- 5f0daae60b15a3157e611770a31e1e3d79ad70e5: Use config yaml for network start (@aefhm)
- 79f73850dd97f6cd9e25f65c59f903a03f76ad65: add test migrations (@ennsharma)
- bea3dbb31523f3f5632b3d0571a425cd1cd7f2fa: address comment (@ennsharma)
- 810ddea0229c8fd53d86e7880c39632916f1ef9a: address comments (@Andrew7234)
- 7f27a7e7567ac8dc105b679f08e9bc5ae1960f59: analyzer: Consensus Main (#36) (@ennsharma)
- 84a602b07ab2769debb7d9d884f9e61b0e74377f: api: Initialize OpenAPI spec (#42) (@ennsharma)
- 81f325788454a3b83a6ac82888f40a572eaaf933: config: Network Configuration (#39) (@ennsharma)
- f9256c33ed0cce1509ef0223c85f1d82147e6a85: fix bad migrations push (@ennsharma)
- dfb781afe35bea8fea2df92bb2c7965c3beb0142: housekeeping rearrangement (@ennsharma)
- e13ee2a54f938bf935668aea3c8a8b690ce60b18: make migrations file flag (#55) (@ennsharma)
- e69b60e9cf9036c94fd15d7458a10406adb578be: make slightly nicer for extension (@ennsharma)
- ed08dc6f947f28d43c7d1e9aa296b70c15bb2ae7: metrics: Add database and request metrics (#52) (@ennsharma)
- c179fd13ca7e677598003ea3116247219df8d925: misc fixes (@Andrew7234)
- 1d4b775ebf6b6d7bc9b7cd2838053a765392b4ec: nit (@Andrew7234)
- 4e25567da38a78709aa2cd8b91114a9150cbdb3e: nit (@ennsharma)
- 4deff8c3f97bb5f019069be049125513cb5a0234: nits (@ennsharma)
- 9a6c3235a34439ad407c99de57b973e815f10863: oasis-indexer: Misc fixes (#56) (@ennsharma)
- b9b1048d8d1b5b6bb41a5b70f5e3ea367e0c819a: oasis-indexer: More repository cleanup (#54) (@ennsharma)
- 6bc9d8acc3991bf07279826823c7eace10989ecb: patch (@ennsharma)
- b005f17e18d1e026791e3c69bfd1190a9a9d518a: remove genesis file (@ennsharma)
- 796627b2df5eddaf548fd60811527fdc53899f1c: remove rpc hardcoding (@ennsharma)
- 30127fc04a222c6b1b56bd67057456cfc38b98c5: rename getters more idiomatically (@ennsharma)
- 58ad4913b72ddff4c0cfcff9fc425163b1a8a815: resolve conflict (@ennsharma)

**Full Changelog**: https://github.com/oasislabs/oasis-indexer/compare/...v0.0.1

### Running the indexer

https://github.com/oasislabs/oasis-indexer/blob/v0.0.1/README.md#docker-development

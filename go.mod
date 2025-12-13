module github.com/oasisprotocol/nexus

go 1.25.3

replace (
	github.com/cometbft/cometbft => github.com/oasisprotocol/cometbft v0.37.2-oasis1
	// v1.5.0 has broken uint parsing, use my commit with fixes instead until
	// the maintainers merge my PR: https://github.com/spf13/cast/pull/144
	github.com/spf13/cast => github.com/oasisprotocol/cast v0.0.0-20220606122631-eba453e69641
	golang.org/x/crypto/curve25519 => github.com/oasisprotocol/curve25519-voi/primitives/x25519 v0.0.0-20210505121811-294cf0fbfb43
	golang.org/x/crypto/ed25519 => github.com/oasisprotocol/curve25519-voi/primitives/ed25519 v0.0.0-20210505121811-294cf0fbfb43
)

require (
	github.com/ethereum/go-ethereum v1.16.7
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-kit/log v0.2.1
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/knadh/koanf v1.5.0
	github.com/prometheus/client_golang v1.23.2
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	golang.org/x/sync v0.19.0
	google.golang.org/grpc v1.77.0
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20251001021608-1fe7b43fc4d6 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230828082145-3c4c8a2d2371 // indirect
	github.com/a8m/envsubst v1.4.2 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.20.0 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.3 // indirect
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0 // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/consensys/gnark-crypto v0.18.1 // indirect
	github.com/cosmos/gogoproto v1.7.0 // indirect
	github.com/crate-crypto/go-eth-kzg v1.4.0 // indirect
	github.com/crate-crypto/go-ipa v0.0.0-20240724233137-53bbb0ceb27a // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/dgraph-io/badger/v4 v4.5.1 // indirect
	github.com/dgraph-io/ristretto/v2 v2.1.0 // indirect
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/ethereum/c-kzg-4844/v2 v2.1.5 // indirect
	github.com/ethereum/go-verkle v0.2.2 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/getkin/kin-openapi v0.133.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-git/go-git/v5 v5.11.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/flatbuffers v24.12.23+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v39 v39.2.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gtank/merlin v0.1.1 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-plugin v1.4.6 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/ipfs/go-cid v0.5.0 // indirect
	github.com/ipfs/go-log/v2 v2.6.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-libp2p v0.44.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mimoo/StrobeGo v0.0.0-20210601165009-122bf33a46e0 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.16.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.1 // indirect
	github.com/multiformats/go-multihash v0.2.3 // indirect
	github.com/multiformats/go-multistream v0.6.1 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/oasisprotocol/deoxysii v0.0.0-20220228165953-2091330c22b7 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/petermattis/goid v0.0.0-20240813172612-4fcff4a6cae7 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/skeema/knownhosts v1.2.1 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/speakeasy-api/jsonpath v0.6.0 // indirect
	github.com/speakeasy-api/openapi-overlay v0.10.2 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/spf13/viper v1.19.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/supranational/blst v0.3.16-0.20250831170142-f48500c1fdbe // indirect
	github.com/tidwall/btree v1.6.0 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	github.com/woodsbury/decimal128 v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.dedis.ch/fixbuf v1.0.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/exp v0.0.0-20250606033433-dcc06ee1d476 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/grpc/security/advancedtls v0.0.0-20221004221323-12db695f1648 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
)

require (
	github.com/akrylysov/pogreb v0.10.2
	github.com/cockroachdb/apd v1.1.0
	github.com/cometbft/cometbft v0.37.16
	github.com/oapi-codegen/oapi-codegen/v2 v2.5.1
	github.com/oapi-codegen/runtime v1.1.2
	github.com/oasisprotocol/curve25519-voi v0.0.0-20251114093237-2ab5a27a1729
	github.com/oasisprotocol/metadata-registry-tools v0.0.0-20240304080528-3218befba9ca
	github.com/oasisprotocol/oasis-core/go v0.2509.0
	github.com/oasisprotocol/oasis-sdk/client-sdk/go v0.16.1-0.20251014092014-fb69325c5e3c
	github.com/rs/cors v1.11.1
	go.dedis.ch/kyber/v3 v3.1.0
	golang.org/x/crypto v0.46.0
)

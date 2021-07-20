module github.com/prysmaticlabs/prysm

go 1.16

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aristanetworks/goarista v0.0.0-20200521140103-6c3304613b30
	github.com/bazelbuild/rules_go v0.23.2
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/cespare/cp v1.1.1 // indirect
	github.com/confluentinc/confluent-kafka-go v1.4.2 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/d4l3k/messagediff v1.2.1
	github.com/deckarep/golang-set v1.7.1 // indirect
	github.com/dgraph-io/ristretto v0.0.4-0.20210318174700-74754f61e018
	github.com/dlclark/regexp2 v1.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/emicklei/dot v0.11.0
	github.com/ethereum/go-ethereum v1.9.25
	github.com/fatih/color v1.9.0 // indirect
	github.com/ferranbt/fastssz v0.0.0-20210719200358-90640294cb9c
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5
	github.com/form3tech-oss/jwt-go v3.2.1+incompatible
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gballet/go-libpcsclite v0.0.0-20191108122812-4678299bea08 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/go-yaml/yaml v2.1.0+incompatible
	github.com/gogo/protobuf v1.3.2
	github.com/golang/gddo v0.0.0-20200528160355-8d077c1d8f4c
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.3
	github.com/google/gofuzz v1.2.0
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.7.3
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.0.1
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/herumi/bls-eth-go-binary v0.0.0-20210130185500-57372fb27371
	github.com/huin/goupnp v1.0.2 // indirect
	github.com/ianlancetaylor/cgosymbolizer v0.0.0-20200424224625-be1b05b0b279
	github.com/ipfs/go-ipfs-addr v0.0.1
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/joonix/log v0.0.0-20200409080653-9c1d2ceb5f1d
	github.com/json-iterator/go v1.1.11
	github.com/k0kubun/go-ansi v0.0.0-20180517002512-3bf9e2903213
	github.com/karalabe/usb v0.0.0-20191104083709-911d15fe12a9 // indirect
	github.com/kevinms/leakybucket-go v0.0.0-20200115003610-082473db97ca
	github.com/klauspost/cpuid/v2 v2.0.8 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/kr/pretty v0.2.1
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-blankhost v0.2.0
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-discovery v0.5.1 // indirect
	github.com/libp2p/go-libp2p-noise v0.2.0
	github.com/libp2p/go-libp2p-peerstore v0.2.7
	github.com/libp2p/go-libp2p-pubsub v0.5.0
	github.com/libp2p/go-libp2p-swarm v0.5.0
	github.com/libp2p/go-libp2p-tls v0.1.4-0.20200421131144-8a8ad624a291 // indirect
	github.com/libp2p/go-tcp-transport v0.2.4
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/manifoldco/promptui v0.7.0
	github.com/miekg/dns v1.1.43 // indirect
	github.com/minio/highwayhash v1.0.1
	github.com/minio/sha256-simd v1.0.0
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/nbutton23/zxcvbn-go v0.0.0-20180912185939-ae427f1e4c1d
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/pborman/uuid v1.2.1
	github.com/peterh/liner v1.2.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prestonvanloon/go-recaptcha v0.0.0-20190217191114-0834cef6e8bd
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.29.0 // indirect
	github.com/prometheus/procfs v0.7.0 // indirect
	github.com/prometheus/prom2json v1.3.0
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/prysmaticlabs/eth2-types v0.0.0-20210303084904-c9735a06829d
	github.com/prysmaticlabs/go-bitfield v0.0.0-20210706153858-5cb5ce8bdbfe
	github.com/prysmaticlabs/prombbolt v0.0.0-20210126082820-9b7adba6db7c
	github.com/prysmaticlabs/protoc-gen-go-cast v0.0.0-20210504233148-1e141af6a0a1
	github.com/r3labs/sse v0.0.0-20210224172625-26fe804710bc
	github.com/rs/cors v1.7.0
	github.com/schollz/progressbar/v3 v3.3.4
	github.com/sirupsen/logrus v1.6.0
	github.com/status-im/keycard-go v0.0.0-20200402102358-957c09536969
	github.com/stretchr/testify v1.7.0
	github.com/supranational/blst v0.3.4
	github.com/thomaso-mirodin/intmath v0.0.0-20160323211736-5dc6d854e46e
	github.com/trailofbits/go-mutexasserts v0.0.0-20200708152505-19999e7d3cef
	github.com/tyler-smith/go-bip39 v1.1.0
	github.com/urfave/cli/v2 v2.3.0
	github.com/wealdtech/go-bytesutil v1.1.1
	github.com/wealdtech/go-eth2-util v1.6.3
	github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4 v1.1.3
	github.com/wercker/journalhook v0.0.0-20180428041537-5d0a5ae867b3
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.23.0
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/automaxprocs v1.3.0
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.18.1 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/exp v0.0.0-20200513190911-00229845015e
	golang.org/x/net v0.0.0-20210716203947-853a461950ff // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
	golang.org/x/tools v0.1.1
	google.golang.org/api v0.34.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210426193834-eac7f76ac494
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/confluentinc/confluent-kafka-go.v1 v1.4.2
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/errgo.v2 v2.1.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/utils v0.0.0-20200520001619-278ece378a50 // indirect
)

replace github.com/ethereum/go-ethereum => github.com/prysmaticlabs/bazel-go-ethereum v0.0.0-20210707101027-e8523651bf6f

replace github.com/json-iterator/go => github.com/prestonvanloon/go v1.1.7-0.20190722034630-4f2e55fcf87b

// See https://github.com/prysmaticlabs/grpc-gateway/issues/2
replace github.com/grpc-ecosystem/grpc-gateway/v2 => github.com/prysmaticlabs/grpc-gateway/v2 v2.3.1-0.20210622145107-ca3041e1b380

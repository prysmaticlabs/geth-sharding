workspace(name = "prysm")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

http_archive(
    name = "bazel_toolchains",
    sha256 = "8e0633dfb59f704594f19ae996a35650747adc621ada5e8b9fb588f808c89cb0",
    strip_prefix = "bazel-toolchains-3.7.0",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-toolchains/releases/download/3.7.0/bazel-toolchains-3.7.0.tar.gz",
        "https://github.com/bazelbuild/bazel-toolchains/releases/download/3.7.0/bazel-toolchains-3.7.0.tar.gz",
    ],
)

http_archive(
    name = "com_grail_bazel_toolchain",
    sha256 = "040b9d00b8a03e8a28e38159ad0f2d0e0de625d93f453a9f226971a8c47e757b",
    strip_prefix = "bazel-toolchain-5f82830f9d6a1941c3eb29683c1864ccf2862454",
    urls = ["https://github.com/grailbio/bazel-toolchain/archive/5f82830f9d6a1941c3eb29683c1864ccf2862454.tar.gz"],
)

load("@com_grail_bazel_toolchain//toolchain:deps.bzl", "bazel_toolchain_dependencies")

bazel_toolchain_dependencies()

load("@com_grail_bazel_toolchain//toolchain:rules.bzl", "llvm_toolchain")

llvm_toolchain(
    name = "llvm_toolchain",
    llvm_version = "10.0.0",
)

load("@llvm_toolchain//:toolchains.bzl", "llvm_register_toolchains")

llvm_register_toolchains()

load("@prysm//tools/cross-toolchain:prysm_toolchains.bzl", "configure_prysm_toolchains")

configure_prysm_toolchains()

load("@prysm//tools/cross-toolchain:rbe_toolchains_config.bzl", "rbe_toolchains_config")

rbe_toolchains_config()

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "bazel_skylib",
    sha256 = "1c531376ac7e5a180e0237938a2536de0c54d93f5c278634818e0efc952dd56c",
    urls = [
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.0.3/bazel-skylib-1.0.3.tar.gz",
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.0.3/bazel-skylib-1.0.3.tar.gz",
    ],
)

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

http_archive(
    name = "bazel_gazelle",
    sha256 = "62ca106be173579c0a167deb23358fdfe71ffa1e4cfdddf5582af26520f1c66f",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.23.0/bazel-gazelle-v0.23.0.tar.gz",
    ],
)

http_archive(
    name = "com_github_atlassian_bazel_tools",
    sha256 = "60821f298a7399450b51b9020394904bbad477c18718d2ad6c789f231e5b8b45",
    strip_prefix = "bazel-tools-a2138311856f55add11cd7009a5abc8d4fd6f163",
    urls = ["https://github.com/atlassian/bazel-tools/archive/a2138311856f55add11cd7009a5abc8d4fd6f163.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "59d5b42ac315e7eadffa944e86e90c2990110a1c8075f1cd145f487e999d22b3",
    strip_prefix = "rules_docker-0.17.0",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.17.0/rules_docker-v0.17.0.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_go",
    patch_args = ["-p1"],
    patches = [
        # Required until https://github.com/bazelbuild/rules_go/pull/2450 merges otherwise nilness
        # nogo check fails for certain third_party dependencies.
        "//third_party:io_bazel_rules_go.patch",
        # Expose internals of go_test for custom build transitions.
        "//third_party:io_bazel_rules_go_test.patch",
    ],
    sha256 = "7c10271940c6bce577d51a075ae77728964db285dac0a46614a7934dc34303e6",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.26.0/rules_go-v0.26.0.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.26.0/rules_go-v0.26.0.tar.gz",
    ],
)

# Override default import in rules_go with special patch until
# https://github.com/gogo/protobuf/pull/582 is merged.
git_repository(
    name = "com_github_gogo_protobuf",
    commit = "b03c65ea87cdc3521ede29f62fe3ce239267c1bc",
    patch_args = ["-p1"],
    patches = [
        "@io_bazel_rules_go//third_party:com_github_gogo_protobuf-gazelle.patch",
        "//third_party:com_github_gogo_protobuf-equal.patch",
    ],
    remote = "https://github.com/gogo/protobuf",
    shallow_since = "1610265707 +0000",
    # gazelle args: -go_prefix github.com/gogo/protobuf -proto legacy
)

http_archive(
    name = "fuzzit_linux",
    build_file_content = "exports_files([\"fuzzit\"])",
    sha256 = "9ca76ac1c22d9360936006efddf992977ebf8e4788ded8e5f9d511285c9ac774",
    urls = ["https://github.com/fuzzitdev/fuzzit/releases/download/v2.4.76/fuzzit_Linux_x86_64.zip"],
)

git_repository(
    name = "graknlabs_bazel_distribution",
    commit = "962f3a7e56942430c0ec120c24f9e9f2a9c2ce1a",
    remote = "https://github.com/graknlabs/bazel-distribution",
    shallow_since = "1569509514 +0300",
)

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)

container_pull(
    name = "alpine_cc_linux_amd64",
    digest = "sha256:752aa0c9a88461ffc50c5267bb7497ef03a303e38b2c8f7f2ded9bebe5f1f00e",
    registry = "index.docker.io",
    repository = "pinglamb/alpine-glibc",
)

container_pull(
    name = "fuzzit_base",
    digest = "sha256:24a39a4360b07b8f0121eb55674a2e757ab09f0baff5569332fefd227ee4338f",
    registry = "gcr.io",
    repository = "fuzzit-public/stretch-llvm8",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.16.4",
    nogo = "@//:nogo",
)

http_archive(
    name = "prysm_testnet_site",
    build_file_content = """
proto_library(
  name = "faucet_proto",
  srcs = ["src/proto/faucet.proto"],
  visibility = ["//visibility:public"],
)""",
    sha256 = "29742136ff9faf47343073c4569a7cf21b8ed138f726929e09e3c38ab83544f7",
    strip_prefix = "prysm-testnet-site-5c711600f0a77fc553b18cf37b880eaffef4afdb",
    url = "https://github.com/prestonvanloon/prysm-testnet-site/archive/5c711600f0a77fc553b18cf37b880eaffef4afdb.tar.gz",
)

http_archive(
    name = "io_kubernetes_build",
    sha256 = "b84fbd1173acee9d02a7d3698ad269fdf4f7aa081e9cecd40e012ad0ad8cfa2a",
    strip_prefix = "repo-infra-6537f2101fb432b679f3d103ee729dd8ac5d30a0",
    url = "https://github.com/kubernetes/repo-infra/archive/6537f2101fb432b679f3d103ee729dd8ac5d30a0.tar.gz",
)

http_archive(
    name = "eip3076_spec_tests",
    build_file_content = """
filegroup(
    name = "test_data",
    srcs = glob([
        "**/*.json",
    ]),
    visibility = ["//visibility:public"],
)
    """,
    sha256 = "91434d5fd5e1c6eb7b0174fed2afe25e09bddf00e1e4c431db931b2cee4e7773",
    url = "https://github.com/eth2-clients/slashing-protection-interchange-tests/archive/b8413ca42dc92308019d0d4db52c87e9e125c4e9.tar.gz",
)

eth2_spec_version = "v1.1.0-beta.1"

http_archive(
    name = "eth2_spec_tests_general",
    build_file_content = """
filegroup(
    name = "test_data",
    srcs = glob([
        "**/*.ssz_snappy",
        "**/*.yaml",
    ]),
    visibility = ["//visibility:public"],
)
    """,
    sha256 = "e9b4cc60a3e676c6b4a9348424e44cff1ebada603ffb31b0df600dbd70e7fbf6",
    url = "https://github.com/ethereum/eth2.0-spec-tests/releases/download/%s/general.tar.gz" % eth2_spec_version,
)

http_archive(
    name = "eth2_spec_tests_minimal",
    build_file_content = """
filegroup(
    name = "test_data",
    srcs = glob([
        "**/*.ssz_snappy",
        "**/*.yaml",
    ]),
    visibility = ["//visibility:public"],
)
    """,
    sha256 = "cf82dc729ffe7b924f852e57d1973e1a6377c5b52acc903c953277fa9b4e6de8",
    url = "https://github.com/ethereum/eth2.0-spec-tests/releases/download/%s/minimal.tar.gz" % eth2_spec_version,
)

http_archive(
    name = "eth2_spec_tests_mainnet",
    build_file_content = """
filegroup(
    name = "test_data",
    srcs = glob([
        "**/*.ssz_snappy",
        "**/*.yaml",
    ]),
    visibility = ["//visibility:public"],
)
    """,
    sha256 = "6c6792375b81858037014e282d28a64b0cf12e12daf16054265c85403b8b329f",
    url = "https://github.com/ethereum/eth2.0-spec-tests/releases/download/%s/mainnet.tar.gz" % eth2_spec_version,
)

http_archive(
    name = "eth2_spec",
    build_file_content = """
filegroup(
    name = "spec_data",
    srcs = glob([
        "**/*.yaml",
    ]),
    visibility = ["//visibility:public"],
)
    """,
    sha256 = "16094dad1bab4e8ab3adb60c10e311cd1e294cd7bbf5a89505f24bebd3d0e513",
    strip_prefix = "eth2.0-specs-" + eth2_spec_version[1:],
    url = "https://github.com/ethereum/eth2.0-specs/archive/refs/tags/%s.tar.gz" % eth2_spec_version,
)

http_archive(
    name = "com_github_bazelbuild_buildtools",
    sha256 = "7a182df18df1debabd9e36ae07c8edfa1378b8424a04561b674d933b965372b3",
    strip_prefix = "buildtools-f2aed9ee205d62d45c55cfabbfd26342f8526862",
    url = "https://github.com/bazelbuild/buildtools/archive/f2aed9ee205d62d45c55cfabbfd26342f8526862.zip",
)

git_repository(
    name = "com_google_protobuf",
    commit = "436bd7880e458532901c58f4d9d1ea23fa7edd52",
    remote = "https://github.com/protocolbuffers/protobuf",
    shallow_since = "1617835118 -0700",
)

# Group the sources of the library so that CMake rule have access to it
all_content = """filegroup(name = "all", srcs = glob(["**"]), visibility = ["//visibility:public"])"""

http_archive(
    name = "rules_foreign_cc",
    sha256 = "b85ce66a3410f7370d1a9a61dfe3a29c7532b7637caeb2877d8d0dfd41d77abb",
    strip_prefix = "rules_foreign_cc-3515b20a2417c4dd51c8a4a8cac1f6ecf3c6d934",
    url = "https://github.com/bazelbuild/rules_foreign_cc/archive/3515b20a2417c4dd51c8a4a8cac1f6ecf3c6d934.zip",
)

load("@rules_foreign_cc//:workspace_definitions.bzl", "rules_foreign_cc_dependencies")

rules_foreign_cc_dependencies([
    "@prysm//:built_cmake_toolchain",
])

http_archive(
    name = "librdkafka",
    build_file_content = all_content,
    sha256 = "3b99a36c082a67ef6295eabd4fb3e32ab0bff7c6b0d397d6352697335f4e57eb",
    strip_prefix = "librdkafka-1.4.2",
    urls = ["https://github.com/edenhill/librdkafka/archive/v1.4.2.tar.gz"],
)

http_archive(
    name = "sigp_beacon_fuzz_corpora",
    build_file = "//third_party:beacon-fuzz/corpora.BUILD",
    sha256 = "42993d0901a316afda45b4ba6d53c7c21f30c551dcec290a4ca131c24453d1ef",
    strip_prefix = "beacon-fuzz-corpora-bac24ad78d45cc3664c0172241feac969c1ac29b",
    urls = [
        "https://github.com/sigp/beacon-fuzz-corpora/archive/bac24ad78d45cc3664c0172241feac969c1ac29b.tar.gz",
    ],
)

# External dependencies

http_archive(
    name = "prysm_web_ui",
    build_file_content = """
filegroup(
    name = "site",
    srcs = glob(["**/*"]),
    visibility = ["//visibility:public"],
)
""",
    sha256 = "54ce527b83d092da01127f2e3816f4d5cfbab69354caba8537f1ea55889b6d7c",
    urls = [
        "https://github.com/prysmaticlabs/prysm-web-ui/releases/download/v1.0.0-beta.4/prysm-web-ui.tar.gz",
    ],
)

load("//:deps.bzl", "prysm_deps")

# gazelle:repository_macro deps.bzl%prysm_deps
prysm_deps()

load("@prysm//third_party/herumi:herumi.bzl", "bls_dependencies")

bls_dependencies()

load(
    "@io_bazel_rules_docker//go:image.bzl",
    _go_image_repos = "repositories",
)

# Golang images
# This is using gcr.io/distroless/base
_go_image_repos()

# CC images
# This is using gcr.io/distroless/base
load(
    "@io_bazel_rules_docker//cc:image.bzl",
    _cc_image_repos = "repositories",
)

_cc_image_repos()

load("@com_github_ethereum_go_ethereum//:deps.bzl", "geth_dependencies")

geth_dependencies()

load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")

go_embed_data_dependencies()

load("@com_github_atlassian_bazel_tools//gometalinter:deps.bzl", "gometalinter_dependencies")

gometalinter_dependencies()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")

gazelle_dependencies()

load("@com_github_bazelbuild_buildtools//buildifier:deps.bzl", "buildifier_dependencies")

buildifier_dependencies()

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()

# Do NOT add new go dependencies here! Refer to DEPENDENCIES.md!

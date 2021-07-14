package helpers

import (
	fssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// ForkVersionByteLength length of fork version byte array.
const ForkVersionByteLength = 4

// DomainByteLength length of domain byte array.
const DomainByteLength = 4

// ErrSigFailedToVerify returns when a signature of a block object(ie attestation, slashing, exit... etc)
// failed to verify.
var ErrSigFailedToVerify = errors.New("signature did not verify")

// ComputeDomainAndSign computes the domain and signing root and sign it using the passed in private key.
func ComputeDomainAndSign(st iface.ReadOnlyBeaconState, epoch types.Epoch, obj fssz.HashRoot, domain [4]byte, key bls.SecretKey) ([]byte, error) {
	d, err := Domain(st.Fork(), epoch, domain, st.GenesisValidatorRoot())
	if err != nil {
		return nil, err
	}
	sr, err := ComputeSigningRoot(obj, d)
	if err != nil {
		return nil, err
	}
	return key.Sign(sr[:]).Marshal(), nil
}

// ComputeSigningRoot computes the root of the object by calculating the hash tree root of the signing data with the given domain.
//
// Spec pseudocode definition:
//	def compute_signing_root(ssz_object: SSZObject, domain: Domain) -> Root:
//    """
//    Return the signing root for the corresponding signing data.
//    """
//    return hash_tree_root(SigningData(
//        object_root=hash_tree_root(ssz_object),
//        domain=domain,
//    ))
func ComputeSigningRoot(object fssz.HashRoot, domain types.Domain) ([32]byte, error) {
	return signingData(object.HashTreeRoot, domain)
}

// Computes the signing data by utilising the provided root function and then
// returning the signing data of the container object.
func signingData(rootFunc func() ([32]byte, error), domain types.Domain) ([32]byte, error) {
	objRoot, err := rootFunc()
	if err != nil {
		return [32]byte{}, err
	}
	container := &pb.SigningData{
		ObjectRoot: objRoot[:],
		Domain:     domain,
	}
	return container.HashTreeRoot()
}

// ComputeDomainVerifySigningRoot computes domain and verifies signing root of an object given the beacon state, validator index and signature.
func ComputeDomainVerifySigningRoot(st iface.BeaconState, index types.ValidatorIndex, epoch types.Epoch, obj fssz.HashRoot, domain [4]byte, sig []byte) error {
	v, err := st.ValidatorAtIndex(index)
	if err != nil {
		return err
	}
	d, err := Domain(st.Fork(), epoch, domain, st.GenesisValidatorRoot())
	if err != nil {
		return err
	}
	return VerifySigningRoot(obj, v.PublicKey, sig, d)
}

// VerifySigningRoot verifies the signing root of an object given it's public key, signature and domain.
func VerifySigningRoot(obj fssz.HashRoot, pub, signature []byte, domain types.Domain) error {
	publicKey, err := bls.PublicKeyFromBytes(pub)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to public key")
	}
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to signature")
	}
	root, err := ComputeSigningRoot(obj, domain)
	if err != nil {
		return errors.Wrap(err, "could not compute signing root")
	}
	if !sig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// VerifyBlockSigningRoot verifies the signing root of a block given it's public key, signature and domain.
func VerifyBlockSigningRoot(pub, signature []byte, domain types.Domain, rootFunc func() ([32]byte, error)) error {
	set, err := BlockSignatureSet(pub, signature, domain, rootFunc)
	if err != nil {
		return err
	}
	// We assume only one signature set is returned here.
	sig := set.Signatures[0]
	publicKey := set.PublicKeys[0]
	root := set.Messages[0]

	rSig, err := bls.SignatureFromBytes(sig)
	if err != nil {
		return err
	}
	if !rSig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// BlockSignatureSet retrieves the relevant signature, message and pubkey data from a block and collating it
// into a signature set object.
func BlockSignatureSet(pub, signature []byte, domain types.Domain, rootFunc func() ([32]byte, error)) (*bls.SignatureSet, error) {
	publicKey, err := bls.PublicKeyFromBytes(pub)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert bytes to public key")
	}
	// utilize custom block hashing function
	root, err := signingData(rootFunc, domain)
	if err != nil {
		return nil, errors.Wrap(err, "could not compute signing root")
	}
	return &bls.SignatureSet{
		Signatures: [][]byte{signature},
		PublicKeys: []bls.PublicKey{publicKey},
		Messages:   [][32]byte{root},
	}, nil
}

// VerifyBlockHeaderSigningRoot verifies the signing root of a block header given it's public key, signature and domain.
func VerifyBlockHeaderSigningRoot(blkHdr *ethpb.BeaconBlockHeader, pub, signature []byte, domain types.Domain) error {
	publicKey, err := bls.PublicKeyFromBytes(pub)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to public key")
	}
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return errors.Wrap(err, "could not convert bytes to signature")
	}
	root, err := signingData(blkHdr.HashTreeRoot, domain)
	if err != nil {
		return errors.Wrap(err, "could not compute signing root")
	}
	if !sig.Verify(publicKey, root[:]) {
		return ErrSigFailedToVerify
	}
	return nil
}

// ComputeDomain returns the domain version for BLS private key to sign and verify with a zeroed 4-byte
// array as the fork version.
//
// def compute_domain(domain_type: DomainType, fork_version: Version=None, genesis_validators_root: Root=None) -> Domain:
//    """
//    Return the domain for the ``domain_type`` and ``fork_version``.
//    """
//    if fork_version is None:
//        fork_version = GENESIS_FORK_VERSION
//    if genesis_validators_root is None:
//        genesis_validators_root = Root()  # all bytes zero by default
//    fork_data_root = compute_fork_data_root(fork_version, genesis_validators_root)
//    return Domain(domain_type + fork_data_root[:28])
func ComputeDomain(domainType [DomainByteLength]byte, forkVersion, genesisValidatorsRoot []byte) (types.Domain, error) {
	if forkVersion == nil {
		forkVersion = params.BeaconConfig().GenesisForkVersion
	}
	if genesisValidatorsRoot == nil {
		genesisValidatorsRoot = params.BeaconConfig().ZeroHash[:]
	}
	forkBytes := [ForkVersionByteLength]byte{}
	copy(forkBytes[:], forkVersion)

	forkDataRoot, err := computeForkDataRoot(forkBytes[:], genesisValidatorsRoot)
	if err != nil {
		return types.Domain{}, err
	}

	return domain(domainType, forkDataRoot[:]), nil
}

// This returns the bls domain given by the domain type and fork data root.
func domain(domainType [DomainByteLength]byte, forkDataRoot []byte) types.Domain {
	var b []byte
	b = append(b, domainType[:4]...)
	b = append(b, forkDataRoot[:28]...)
	b32 := bytesutil.ToBytes32(b)
	return b32[:]
}

// this returns the 32byte fork data root for the ``current_version`` and ``genesis_validators_root``.
// This is used primarily in signature domains to avoid collisions across forks/chains.
//
// Spec pseudocode definition:
//	def compute_fork_data_root(current_version: Version, genesis_validators_root: Root) -> Root:
//    """
//    Return the 32-byte fork data root for the ``current_version`` and ``genesis_validators_root``.
//    This is used primarily in signature domains to avoid collisions across forks/chains.
//    """
//    return hash_tree_root(ForkData(
//        current_version=current_version,
//        genesis_validators_root=genesis_validators_root,
//    ))
func computeForkDataRoot(version, root []byte) ([32]byte, error) {
	r, err := (&pb.ForkData{
		CurrentVersion:        version,
		GenesisValidatorsRoot: root,
	}).HashTreeRoot()
	if err != nil {
		return [32]byte{}, err
	}
	return r, nil
}

// ComputeForkDigest returns the fork for the current version and genesis validator root
//
// Spec pseudocode definition:
//	def compute_fork_digest(current_version: Version, genesis_validators_root: Root) -> ForkDigest:
//    """
//    Return the 4-byte fork digest for the ``current_version`` and ``genesis_validators_root``.
//    This is a digest primarily used for domain separation on the p2p layer.
//    4-bytes suffices for practical separation of forks/chains.
//    """
//    return ForkDigest(compute_fork_data_root(current_version, genesis_validators_root)[:4])
func ComputeForkDigest(version, genesisValidatorsRoot []byte) ([4]byte, error) {
	dataRoot, err := computeForkDataRoot(version, genesisValidatorsRoot)
	if err != nil {
		return [4]byte{}, err
	}
	return bytesutil.ToBytes4(dataRoot[:]), nil
}

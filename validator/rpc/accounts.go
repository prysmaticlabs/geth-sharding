package rpc

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	pb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/pagination"
	"github.com/prysmaticlabs/prysm/shared/petnames"
	v2 "github.com/prysmaticlabs/prysm/validator/accounts/v2"
	v2keymanager "github.com/prysmaticlabs/prysm/validator/keymanager/v2"
	"github.com/prysmaticlabs/prysm/validator/keymanager/v2/derived"
	"github.com/prysmaticlabs/prysm/validator/keymanager/v2/direct"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type accountCreator interface {
	CreateAccount(ctx context.Context) ([]byte, *ethpb.Deposit_Data, error)
}

// CreateAccount allows creation of a new account in a user's wallet via RPC.
func (s *Server) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.DepositDataResponse, error) {
	if !s.walletInitialized {
		return nil, status.Error(codes.FailedPrecondition, "Wallet not yet initialized")
	}
	var creator accountCreator
	switch s.wallet.KeymanagerKind() {
	case v2keymanager.Remote:
		return nil, status.Error(codes.InvalidArgument, "Cannot create account for remote keymanager")
	case v2keymanager.Direct:
		km, ok := s.keymanager.(*direct.Keymanager)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "Not a direct keymanager")
		}
		creator = km
	case v2keymanager.Derived:
		km, ok := s.keymanager.(*derived.Keymanager)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "Not a derived keymanager")
		}
		creator = km
	}
	dataList := make([]*pb.DepositDataResponse_DepositData, req.NumAccounts)
	for i := uint64(0); i < req.NumAccounts; i++ {
		data, err := createAccountWithDepositData(ctx, creator)
		if err != nil {
			return nil, err
		}
		dataList[i] = data
	}
	return &pb.DepositDataResponse{
		DepositDataList: dataList,
	}, nil
}

// ListAccounts allows retrieval of validating keys and their petnames
// for a user's wallet via RPC.
func (s *Server) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	if !s.walletInitialized {
		return nil, status.Error(codes.FailedPrecondition, "Wallet not yet initialized")
	}
	if int(req.PageSize) > cmd.Get().MaxRPCPageSize {
		return nil, status.Errorf(codes.InvalidArgument, "Requested page size %d can not be greater than max size %d",
			req.PageSize, cmd.Get().MaxRPCPageSize)
	}
	keys, err := s.keymanager.FetchValidatingPublicKeys(ctx)
	if err != nil {
		return nil, err
	}
	accounts := make([]*pb.Account, len(keys))
	for i := 0; i < len(keys); i++ {
		accounts[i] = &pb.Account{
			ValidatingPublicKey: keys[i][:],
			AccountName:         petnames.DeterministicName(keys[i][:], "-"),
		}
		if s.wallet.KeymanagerKind() == v2keymanager.Derived {
			accounts[i].DerivationPath = fmt.Sprintf(derived.ValidatingKeyDerivationPathTemplate, i)
		}
	}
	if req.All {
		return &pb.ListAccountsResponse{
			Accounts:      accounts,
			TotalSize:     int32(len(keys)),
			NextPageToken: "",
		}, nil
	}
	start, end, nextPageToken, err := pagination.StartAndEndPage(req.PageToken, int(req.PageSize), len(keys))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"Could not paginate results: %v",
			err,
		)
	}
	return &pb.ListAccountsResponse{
		Accounts:      accounts[start:end],
		TotalSize:     int32(len(keys)),
		NextPageToken: nextPageToken,
	}, nil
}

// BackupAccounts creates a zip file containing EIP-2335 keystores for the user's
// specified public keys by encrypting them with the specified password.
func (s *Server) BackupAccounts(
	ctx context.Context, req *pb.BackupAccountsRequest,
) (*pb.BackupAccountsResponse, error) {
	if req.PublicKeys == nil || len(req.PublicKeys) < 1 {
		return nil, status.Error(codes.InvalidArgument, "No public keys specified to backup")
	}
	if req.BackupPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "Backup password cannot be empty")
	}
	if s.wallet == nil || s.keymanager == nil {
		return nil, status.Error(codes.FailedPrecondition, "No wallet nor keymanager found")
	}
	if s.wallet.KeymanagerKind() != v2keymanager.Direct && s.wallet.KeymanagerKind() != v2keymanager.Derived {
		return nil, status.Error(codes.FailedPrecondition, "Only HD or direct wallets can backup accounts")
	}
	pubKeys := make([]bls.PublicKey, len(req.PublicKeys))
	for i, key := range req.PublicKeys {
		pubKey, err := bls.PublicKeyFromBytes(key)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%#x Not a valid BLS public key: %v", key, err)
		}
		pubKeys[i] = pubKey
	}
	var keystoresToBackup []*v2keymanager.Keystore
	var err error
	switch s.wallet.KeymanagerKind() {
	case v2keymanager.Direct:
		km, ok := s.keymanager.(*direct.Keymanager)
		if !ok {
			return nil, status.Error(codes.FailedPrecondition, "Could not assert keymanager interface to concrete type")
		}
		keystoresToBackup, err = km.ExtractKeystores(ctx, pubKeys, req.BackupPassword)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not backup accounts for direct keymanager: %v", err)
		}
	case v2keymanager.Derived:
		km, ok := s.keymanager.(*derived.Keymanager)
		if !ok {
			return nil, status.Error(codes.FailedPrecondition, "Could not assert keymanager interface to concrete type")
		}
		keystoresToBackup, err = km.ExtractKeystores(ctx, pubKeys, req.BackupPassword)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Could not backup accounts for derived keymanager: %v", err)
		}
	}
	if len(keystoresToBackup) == 0 {
		return nil, status.Error(codes.InvalidArgument, "No keystores to backup")
	}

	buf := new(bytes.Buffer)
	writer := zip.NewWriter(buf)
	for i, k := range keystoresToBackup {
		encodedFile, err := json.MarshalIndent(k, "", "\t")
		if err != nil {
			if err := writer.Close(); err != nil {
				log.WithError(err).Error("Could not close zip file after writing")
			}
			return nil, status.Errorf(codes.Internal, "could not marshal keystore to JSON file: %v", err)
		}
		f, err := writer.Create(fmt.Sprintf("keystore-%d.json", i))
		if err != nil {
			if err := writer.Close(); err != nil {
				log.WithError(err).Error("Could not close zip file after writing")
			}
			return nil, status.Errorf(codes.Internal, "Could not write keystore file to zip: %v", err)
		}
		if _, err = f.Write(encodedFile); err != nil {
			if err := writer.Close(); err != nil {
				log.WithError(err).Error("Could not close zip file after writing")
			}
			return nil, status.Errorf(codes.Internal, "Could not write keystore file contents")
		}
	}
	if err := writer.Close(); err != nil {
		log.WithError(err).Error("Could not close zip file after writing")
	}
	return &pb.BackupAccountsResponse{
		ZipFile: buf.Bytes(),
	}, nil
}

// DeleteAccounts deletes accounts from a user if their wallet is a non-HD wallet.
func (s *Server) DeleteAccounts(
	ctx context.Context, req *pb.DeleteAccountsRequest,
) (*pb.DeleteAccountsResponse, error) {
	if req.PublicKeys == nil || len(req.PublicKeys) < 1 {
		return nil, status.Error(codes.InvalidArgument, "No public keys specified to delete")
	}
	if s.wallet == nil || s.keymanager == nil {
		return nil, status.Error(codes.FailedPrecondition, "No wallet nor keymanager found")
	}
	if s.wallet.KeymanagerKind() != v2keymanager.Direct {
		return nil, status.Error(codes.FailedPrecondition, "Only Non-HD wallets can delete accounts")
	}
	if err := v2.DeleteAccount(ctx, &v2.DeleteAccountConfig{
		Wallet:     s.wallet,
		Keymanager: s.keymanager,
		PublicKeys: req.PublicKeys,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "Could not delete public keys: %v", err)
	}
	return &pb.DeleteAccountsResponse{
		DeletedKeys: req.PublicKeys,
	}, nil
}

func createAccountWithDepositData(ctx context.Context, km accountCreator) (*pb.DepositDataResponse_DepositData, error) {
	// Create a new validator account using the specified keymanager.
	_, depositData, err := km.CreateAccount(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not create account in wallet")
	}
	data, err := v2.DepositDataJSON(depositData)
	if err != nil {
		return nil, errors.Wrap(err, "could not create deposit data JSON")
	}
	return &pb.DepositDataResponse_DepositData{
		Data: data,
	}, nil
}

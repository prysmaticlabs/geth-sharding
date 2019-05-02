package powchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"go.opencensus.io/trace"
)

// BlockExists returns true if the block exists, it's height and any possible error encountered.
func (w *Web3Service) BlockExists(ctx context.Context, hash common.Hash) (bool, *big.Int, error) {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.web3service.BlockExists")
	defer span.End()

	if exists, blkInfo, err := w.blockCache.BlockInfoByHash(hash); exists || err != nil {
		if err != nil {
			return false, nil, err
		}
		span.AddAttributes(trace.BoolAttribute("blockCacheHit", true))
		return true, blkInfo.Number, nil
	}
	span.AddAttributes(trace.BoolAttribute("blockCacheHit", false))
	block, err := w.blockFetcher.BlockByHash(ctx, hash)
	if err != nil {
		return false, big.NewInt(0), fmt.Errorf("could not query block with given hash: %v", err)
	}

	if err := w.blockCache.AddBlock(block); err != nil {
		return false, big.NewInt(0), err
	}

	return true, block.Number(), nil
}

// BlockHashByHeight returns the block hash of the block at the given height.
func (w *Web3Service) BlockHashByHeight(ctx context.Context, height *big.Int) (common.Hash, error) {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.web3service.BlockHashByHeight")
	defer span.End()

	if exists, blkInfo, err := w.blockCache.BlockInfoByHeight(height); exists || err != nil {
		if err != nil {
			return [32]byte{}, err
		}
		span.AddAttributes(trace.BoolAttribute("blockCacheHit", true))
		return blkInfo.Hash, nil
	}
	span.AddAttributes(trace.BoolAttribute("blockCacheHit", false))
	block, err := w.blockFetcher.BlockByNumber(w.ctx, height)
	if err != nil {
		return [32]byte{}, fmt.Errorf("could not query block with given height: %v", err)
	}
	if err := w.blockCache.AddBlock(block); err != nil {
		return [32]byte{}, err
	}
	return block.Hash(), nil
}

// BlockByHeight fetches an eth1.0 block by its height.
func (w *Web3Service) BlockByHeight(ctx context.Context, height *big.Int) (*gethTypes.Block, error) {
	ctx, span := trace.StartSpan(ctx, "beacon-chain.web3service.BlockByHeight")
	defer span.End()
	block, err := w.blockFetcher.BlockByNumber(w.ctx, height)
	if err != nil {
		return nil, fmt.Errorf("could not query block with given height: %v", err)
	}
	return block, nil
}

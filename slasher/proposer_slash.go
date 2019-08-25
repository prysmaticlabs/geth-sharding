// Package slasher implements slashing validation
// and proof creation.
package slasher

import (
	"errors"

	"github.com/prysmaticlabs/go-bitfield"
)

var epochProposalBitlist map[uint64]bitfield.Bitlist
var currentEpoch, weakSubjectivityPeriod uint64
var epochs []uint64

func init() {
	epochProposalBitlist = make(map[uint64]bitfield.Bitlist)
	weakSubjectivityPeriod = uint64(54000)
}

// CheckNewProposal checks weather a new proposal is allowed or
// creating a slashable event.
// returns true if it is the first time this
// validatorID propose a block in this epoch or not.
func CheckNewProposal(currentEpoch uint64, epoch uint64, validatorID uint64) (bool, error) {
	if currentEpoch > weakSubjectivityPeriod && epoch < currentEpoch-weakSubjectivityPeriod {
		return false, errors.New("epoch is obsolete = before weak subjectivity period")
	}

	if _, ok := epochProposalBitlist[epoch]; !ok {
		epochProposalBitlist[epoch] = bitfield.NewBitlist(300000)
		epochs = insertSort(epochs, epoch)
		var truncate bool
		var itemsToTruncate []uint64
		if currentEpoch > weakSubjectivityPeriod {
			truncate, epochs, itemsToTruncate = truncateItems(epochs, currentEpoch-weakSubjectivityPeriod)
			if truncate {
				for _, key := range itemsToTruncate {
					delete(epochProposalBitlist, key)
				}
			}
		}
	}
	proposalExists := epochProposalBitlist[epoch].BitAt(validatorID)
	if !proposalExists {
		epochProposalBitlist[epoch].SetBitAt(validatorID, true)
		return true, nil
	}
	return false, nil
}

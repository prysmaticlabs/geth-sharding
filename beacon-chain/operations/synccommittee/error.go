package synccommittee

import "github.com/pkg/errors"

var (
	nilMessageErr      = errors.New("sync committee message is nil")
	nilContributionErr = errors.New("sync committee contribution is nil")
)

package types

// ChunkKind to differentiate what kind of span we are working
// with for slashing detection, either min or max span.
type ChunkKind uint

const (
	MinSpan ChunkKind = iota
	MaxSpan
)

// CompactAttestation containing only the required information
// for attester slashing detection.
type CompactAttestation struct {
	AttestingIndices []uint64
	Source           uint64
	Target           uint64
	SigningRoot      [32]byte
}

// SlashingKind is an enum representing the type of slashable
// offense detected by slasher, useful for conditionals or for logging.
type SlashingKind int

const (
	NotSlashable SlashingKind = iota
	DoubleVote
	SurroundingVote
	SurroundedVote
)

func (k SlashingKind) String() string {
	switch k {
	case NotSlashable:
		return "NOT_SLASHABLE"
	case DoubleVote:
		return "DOUBLE_VOTE"
	case SurroundingVote:
		return "SURROUNDING_VOTE"
	case SurroundedVote:
		return "SURROUNDED_VOTE"
	default:
		return "UNKNOWN"
	}
}

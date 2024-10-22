package types

type TimeLockTxType string

const (
	InclusionProofReceivedTxType TimeLockTxType = "INCLUSION_PROOF_RECEIVED"
)

func (t TimeLockTxType) String() string {
	return string(t)
}

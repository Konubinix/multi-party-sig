package sign

import (
	"github.com/taurusgroup/cmp-ecdsa/pkg/message"
	"github.com/taurusgroup/cmp-ecdsa/pkg/party"
	"github.com/taurusgroup/cmp-ecdsa/pkg/round"
)

// This round sort of corresponds with Figure 2 of the Frost paper:
//   https://eprint.iacr.org/2020/852.pdf
//
// The main difference is that instead of having a separate pre-processing step,
// we instead have an additional round at the start of the signing step.
// The goal of this round is to generate two nonces, and corresponding commitments.
//
// There are also differences corresponding to the lack of a signing authority,
// namely that these commitments are broadcast, instead of stored with the authority.
type round1 struct {
	*round.Helper
	// M is the hash of the message we're signing.
	//
	// This plays the same role as m in the Frost paper. One slight difference
	// is that instead of including the message directly in various hashes,
	// we include the *hash* of that message instead. This provides the same
	// security.
	M []byte
}

// ProcessMessage implements round.Round.
func (r *round1) ProcessMessage(party.ID, message.Content) error { return nil }

// Finalize implements round.Round.
func (r *round1) Finalize(out chan<- *message.Message) (round.Round, error) {
	panic("unimplemented")
}

// MessageContent implements round.Round.
//
// Since this is the first round of the protocol, we expect to see a dummy First type.
func (r *round1) MessageContent() message.Content {
	return &message.First{}
}

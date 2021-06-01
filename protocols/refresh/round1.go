package refresh

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/taurusgroup/cmp-ecdsa/pb"
	"github.com/taurusgroup/cmp-ecdsa/pkg/hash"
	"github.com/taurusgroup/cmp-ecdsa/pkg/math/curve"
	"github.com/taurusgroup/cmp-ecdsa/pkg/math/polynomial"
	"github.com/taurusgroup/cmp-ecdsa/pkg/paillier"
	"github.com/taurusgroup/cmp-ecdsa/pkg/params"
	"github.com/taurusgroup/cmp-ecdsa/pkg/party"
	"github.com/taurusgroup/cmp-ecdsa/pkg/round"
)

type round1 struct {
	*round.BaseRound

	// keygen determines whether we are creating new key material or refreshing
	keygen bool

	thisParty *localParty
	parties   map[party.ID]*localParty

	// decommitment of the 2nd message
	decommitment hash.Decommitment // uᵢ

	// lambda is λᵢ used to generate the Pedersen parameters
	lambda *big.Int

	// poly is fᵢ(X)
	poly *polynomial.Polynomial

	// schnorrRand is an array to t+1 random aₗ ∈ 𝔽 used to compute Schnorr commitments of
	// the coefficients of the exponent polynomial Fᵢ(X)
	schnorrRand []*curve.Scalar
}

// ProcessMessage implements round.Round
func (round *round1) ProcessMessage(*pb.Message) error {
	// In the first round, no messages are expected.
	return nil
}

// GenerateMessages implements round.Round
//
// - sample { aₗ }ₗ  <- 𝔽 for l = 0, ..., t
// - set { Aᵢ = aₗ⋅G}ₗ for l = 0, ..., t
// - sample Paillier pᵢ, qᵢ
// - sample Pedersen Nᵢ, Sᵢ, Tᵢ
// - sample fᵢ(X) <- 𝔽[X], deg(fᵢ) = t
//   - if keygen, fᵢ(0) = xᵢ (additive share of full ECDSA secret key)
//   - if refresh, fᵢ(0) = 0
// - compute Fᵢ(X) = fᵢ(X)⋅G
// - sample rhoᵢ <- {0,1}ᵏ
//   - if keygen, this is RIDᵢ
//   - if refresh, this is used to bind the zk proof to a random value
// - commit to message
func (round *round1) GenerateMessages() ([]*pb.Message, error) {
	var err error

	// generate Schnorr randomness and commitments
	round.thisParty.A = make([]*curve.Point, round.S.Threshold+1)
	round.schnorrRand = make([]*curve.Scalar, round.S.Threshold+1)
	for i := range round.thisParty.A {
		round.schnorrRand[i] = curve.NewScalarRandom()
		round.thisParty.A[i] = curve.NewIdentityPoint().ScalarBaseMult(round.schnorrRand[i])
	}

	// generate Paillier and Pedersen
	skPaillier := paillier.NewSecretKey()
	pkPaillier := skPaillier.PublicKey()
	round.thisParty.Pedersen, round.lambda = skPaillier.GeneratePedersen()
	round.S.Secret.Paillier = skPaillier
	round.thisParty.Paillier = pkPaillier

	// sample fᵢ(X) deg(fᵢ) = t, fᵢ(0) = constant
	// if keygen then constant = secret, otherwise it is 0
	constant := curve.NewScalar()
	if round.keygen {
		constant = curve.NewScalarRandom()
	}
	round.poly = polynomial.NewPolynomial(round.S.Threshold, constant)
	selfScalar := curve.NewScalar().SetBytes([]byte(round.SelfID))
	round.thisParty.shareReceived = round.poly.Evaluate(selfScalar)

	// set Fᵢ(X) = fᵢ(X)•G
	round.thisParty.polyExp = polynomial.NewPolynomialExponent(round.poly)

	// Sample ρᵢ
	round.thisParty.rho = make([]byte, params.SecBytes)
	if _, err = rand.Read(round.thisParty.rho); err != nil {
		return nil, fmt.Errorf("refresh.round1.GenerateMessages(): sample rho: %w", err)
	}

	// commit to data in message 2
	round.thisParty.commitment, round.decommitment, err = round.H.Commit(round.SelfID,
		round.thisParty.rho, round.thisParty.polyExp, round.thisParty.A, round.thisParty.Pedersen)
	if err != nil {
		return nil, fmt.Errorf("refresh.round1.GenerateMessages(): commit: %w", err)
	}

	return []*pb.Message{{
		Type:      pb.MessageType_TypeRefresh1,
		From:      round.SelfID,
		Broadcast: pb.Broadcast_Reliable,
		Refresh1: &pb.Refresh1{
			Hash: round.thisParty.commitment,
		},
	}}, nil
}

// Finalize implements round.Round
func (round *round1) Finalize() (round.Round, error) {
	return &round2{round, nil}, nil
}

func (round *round1) MessageType() pb.MessageType {
	return pb.MessageType_TypeInvalid
}

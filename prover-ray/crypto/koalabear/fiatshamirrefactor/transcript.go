package fiatshamirrefactor

import (
	"errors"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
)

// errChallengeNotFound is returned when a wrong challenge name is provided.
var (
	errChallengeNotFound            = errors.New("challenge not recorded in the transcript")
	errChallengeAlreadyComputed     = errors.New("challenge already computed, cannot be binded to other values")
	errPreviousChallengeNotComputed = errors.New("the previous challenge is needed and has not been computed")
	errChallengeAlreadyExists       = errors.New("this challenge name is already used and recorded")
	errInvalidGrindingBits          = errors.New("invalid grinding bit count")
	errInvalidProofOfWork           = errors.New("invalid proof of work")
)

const (
	challengeIDDomainTag uint64 = 0x46534944 // "FSID"
	proofOfWorkDomainTag uint64 = 0x46535057 // "FSPW"
	koalabearBits               = 31
	maxGrindingBits             = koalabearBits // current proofs of work use a single Koalabear salt
)

// ComputeChallengeConfig configuration file used when 'ComputeChallenge' is called
type ComputeChallengeConfig struct {
	Grinding int // number of bits of grinding
}

type ComputeChallengeOption func(config *ComputeChallengeConfig) error

// WithGrinding asks ComputeChallenge to derive the challenge with a proof-of-work
// salt such that the first nbBits sampled bits are zero.
func WithGrinding(nbBits int) ComputeChallengeOption {
	return func(config *ComputeChallengeConfig) error {
		config.Grinding = nbBits
		return nil
	}
}

// ProofOfWork contains the witness proving that the prover found a salt
// ensuring that the challenge's first NbBits are zeroes.
type ProofOfWork struct {
	NbBits int
	Salt   koalabear.Element
}

// Transcript implements a Fiat-Shamir transcript for transforming
// an interactive protocol into a non-interactive one.
// Challenges must be computed in the order they were registered.
type Transcript struct {
	// hash function that is used.
	h hash.FieldHasher

	challenges         []challenge // the order matters
	nameToChallengePos map[string]int
	// proofOfWork[<name>] -> pow for challenge <name> (if there has been grinding)
	proofOfWork map[string]ProofOfWork
}

type challenge struct {
	bindings   []koalabear.Element // bindings stores the variables a challenge is binded to.
	name       string
	value      hash.Digest // value stores the computed challenge
	isComputed bool
}

// NewTranscript creates a new Fiat-Shamir transcript using the given hash function.
// challengesID are the names of the challenges that will be computed; the order
// matters, as each challenge depends on the previous one.
// Additional challenges can be appended later with [Transcript.NewChallenge].
//
// It panics if duplicate challenge names are provided.
func NewTranscript(h hash.FieldHasher, challengesID ...string) *Transcript {
	t := &Transcript{
		challenges:         make([]challenge, 0, len(challengesID)),
		nameToChallengePos: make(map[string]int, len(challengesID)),
		proofOfWork:        make(map[string]ProofOfWork),
		h:                  h,
	}
	for _, id := range challengesID {
		if _, ok := t.nameToChallengePos[id]; ok {
			panic("duplicate challenge name: " + id)
		}
		t.nameToChallengePos[id] = len(t.challenges)
		t.challenges = append(t.challenges, challenge{name: id})
	}
	return t
}

// Bind binds a value to the given challenge. A challenge can be bound to an
// arbitrary number of values, but the order in which the values are added
// matters. It returns an error if the challenge does not exist or has already
// been computed.
func (t *Transcript) Bind(challengeID string, bValue []koalabear.Element) error {

	pos, ok := t.nameToChallengePos[challengeID]
	if !ok {
		return errChallengeNotFound
	}

	currentChallenge := t.challenges[pos]
	if currentChallenge.isComputed {
		return errChallengeAlreadyComputed
	}

	bCopy := make([]koalabear.Element, len(bValue))
	copy(bCopy, bValue)
	currentChallenge.bindings = append(currentChallenge.bindings, bCopy...)
	t.challenges[pos] = currentChallenge

	return nil
}

// NewChallenge appends a new challenge to the transcript. The newly added
// challenge becomes the last in the computation order. It returns an error if a
// challenge with the same name already exists.
func (t *Transcript) NewChallenge(challengeID string) error {
	if _, ok := t.nameToChallengePos[challengeID]; ok {
		return errChallengeAlreadyExists
	}
	nbChallenges := len(t.challenges)
	challenge := challenge{
		name:       challengeID,
		isComputed: false,
	}
	t.challenges = append(t.challenges, challenge)
	t.nameToChallengePos[challengeID] = nbChallenges
	return nil
}

// ProofOfWork returns the proof of work recorded for the given challenge.
func (t *Transcript) ProofOfWork(challengeID string) (ProofOfWork, bool) {
	pow, ok := t.proofOfWork[challengeID]
	return pow, ok
}

// SetProofOfWork records the proof of work to use when replaying a challenge.
func (t *Transcript) SetProofOfWork(challengeID string, pow ProofOfWork) error {
	if _, ok := t.nameToChallengePos[challengeID]; !ok {
		return errChallengeNotFound
	}
	if err := validateGrindingBits(pow.NbBits); err != nil {
		return err
	}
	if t.proofOfWork == nil {
		t.proofOfWork = make(map[string]ProofOfWork)
	}
	t.proofOfWork[challengeID] = pow
	return nil
}

// ProofsOfWork returns a copy of the proofs of work recorded in the transcript.
func (t *Transcript) ProofsOfWork() map[string]ProofOfWork {
	res := make(map[string]ProofOfWork, len(t.proofOfWork))
	for challengeID, pow := range t.proofOfWork {
		res[challengeID] = pow
	}
	return res
}

// ComputeChallenge computes and returns the challenge corresponding to the
// given name. Challenges must be computed sequentially in the order they were
// registered. The result is:
//   - H(name || bound_values...) for the first challenge
//   - H(name || previous_challenge || bound_values...) for subsequent challenges
//
// If the challenge has already been computed, the cached value is returned.
// It returns an error if the challenge does not exist or if the previous
// challenge has not been computed yet.
func (t *Transcript) ComputeChallenge(
	challengeID string,
	opts ...ComputeChallengeOption,
) ([8]koalabear.Element, error) {

	var config ComputeChallengeConfig
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return [8]koalabear.Element{}, err
		}
	}
	if err := validateGrindingBits(config.Grinding); err != nil {
		return [8]koalabear.Element{}, err
	}

	pos, ok := t.nameToChallengePos[challengeID]
	if !ok {
		return [8]koalabear.Element{}, errChallengeNotFound
	}

	// if the challenge was already computed we return it
	challenge := t.challenges[pos]
	if challenge.isComputed {
		if config.Grinding != 0 {
			pow, ok := t.proofOfWork[challengeID]
			if !ok || pow.NbBits != config.Grinding || !hasZeroGrindingBits(challenge.value, config.Grinding) {
				return [8]koalabear.Element{}, errInvalidProofOfWork
			}
		}
		return challenge.value, nil
	}

	var value hash.Digest
	var err error
	if config.Grinding == 0 {
		value, err = t.computeChallengeDigest(challengeID, pos, challenge, nil)
	} else if pow, ok := t.proofOfWork[challengeID]; ok {
		if pow.NbBits != config.Grinding {
			return [8]koalabear.Element{}, errInvalidProofOfWork
		}
		value, err = t.computeChallengeDigest(challengeID, pos, challenge, &pow)
		if err == nil && !hasZeroGrindingBits(value, config.Grinding) {
			return [8]koalabear.Element{}, errInvalidProofOfWork
		}
	} else {
		var pow ProofOfWork
		pow, value, err = t.grindChallenge(challengeID, pos, challenge, config.Grinding)
		if err == nil {
			if t.proofOfWork == nil {
				t.proofOfWork = make(map[string]ProofOfWork)
			}
			t.proofOfWork[challengeID] = pow
		}
	}
	if err != nil {
		return [8]koalabear.Element{}, err
	}

	challenge.value = value
	challenge.isComputed = true

	t.challenges[pos] = challenge

	return challenge.value, nil

}

func (t *Transcript) computeChallengeDigest(
	challengeID string,
	pos int,
	challenge challenge,
	pow *ProofOfWork,
) (hash.Digest, error) {
	t.h.Reset()

	t.h.WriteElements(hash.StringToElements(challengeIDDomainTag, challengeID)...)

	// write the previous challenge if it's not the first challenge
	if pos != 0 {
		if !t.challenges[pos-1].isComputed {
			t.h.Reset()
			return hash.Digest{}, errPreviousChallengeNotComputed
		}
		t.h.WriteElements(t.challenges[pos-1].value[:]...)
	}

	// write the binded values in the order they were added
	for _, b := range challenge.bindings {
		t.h.WriteElements(b)
	}

	if pow != nil {
		t.h.WriteElements(
			hash.NewElement(proofOfWorkDomainTag),
			hash.NewElement(uint64(pow.NbBits)),
			pow.Salt,
		)
	}

	value := t.h.Sum()
	t.h.Reset()
	return value, nil
}

func (t *Transcript) grindChallenge(
	challengeID string,
	pos int,
	challenge challenge,
	nbBits int,
) (ProofOfWork, hash.Digest, error) {
	modulus := koalabear.Modulus().Uint64()
	for saltValue := uint64(0); saltValue < modulus; saltValue++ {
		pow := ProofOfWork{NbBits: nbBits}
		pow.Salt.SetUint64(saltValue)

		value, err := t.computeChallengeDigest(challengeID, pos, challenge, &pow)
		if err != nil {
			return ProofOfWork{}, hash.Digest{}, err
		}
		if hasZeroGrindingBits(value, nbBits) {
			return pow, value, nil
		}
	}
	return ProofOfWork{}, hash.Digest{}, errInvalidProofOfWork
}

func validateGrindingBits(nbBits int) error {
	if nbBits < 0 || nbBits > maxGrindingBits {
		return errInvalidGrindingBits
	}
	return nil
}

func hasZeroGrindingBits(challenge hash.Digest, nbBits int) bool {
	remaining := nbBits
	for i := 0; i < hash.ExtDegree && remaining > 0; i++ {
		nbBitsToCheck := remaining
		if nbBitsToCheck > koalabearBits {
			nbBitsToCheck = koalabearBits
		}
		mask := uint64(1<<nbBitsToCheck) - 1
		if challenge[i].Uint64()&mask != 0 {
			return false
		}
		remaining -= nbBitsToCheck
	}
	return true
}

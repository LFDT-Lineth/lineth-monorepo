package test_utils

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/consensys/gnark/frontend"
	snarkHash "github.com/consensys/gnark/std/hash"
	"github.com/consensys/linea-monorepo/prover/backend/execution"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

type FakeTestingT struct{}

func (FakeTestingT) Errorf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format+"\n", args...))
}

func (FakeTestingT) FailNow() {
	os.Exit(-1)
}

func RandIntN(n int) int {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	if n > math.MaxInt {
		panic("RandIntN: n too large")
	}
	return int(binary.BigEndian.Uint64(b[:]) % uint64(n)) // #nosec G115 -- Above check precludes an overflow
}

func RandIntSliceN(length, n int) []int {
	res := make([]int, length)
	for i := range res {
		res[i] = RandIntN(n)
	}
	return res
}

// WriterHash is a wrapper around a hash.Hash that duplicates all writes.
type WriterHash struct {
	h hash.Hash
	w io.Writer
}

// ReaderHash is a wrapper around a hash.Hash that matches all writes with its input stream.
type ReaderHash struct {
	h hash.Hash
	r io.Reader
}

// ReaderHashSnark is a wrapper around a FieldHasher that matches all writes with its input stream.
type ReaderHashSnark struct {
	h   snarkHash.FieldHasher
	r   io.Reader
	api frontend.API
}

// GetRepoRootPath returns the monorepo root by walking up from the current
// working directory until it finds the .git marker. Independent of the
// checkout directory name.
func GetRepoRootPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find repo root")
		}
		dir = parent
	}
}

// GetZkevmWitness returns a [zkevm.Witness]
func GetZkevmWitness(req *execution.Request, cfg *config.Config) (*execution.Response, *zkevm.Witness) {
	out := execution.CraftProverOutput(cfg, req)
	witness := execution.NewWitness(cfg, req, &out)
	return &out, witness.ZkEVM
}

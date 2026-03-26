package keccak

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"golang.org/x/crypto/sha3"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	in                [][]byte
	hash              [][32]byte
	lanes             [][8]byte
	isFirstLaneOfHash []int
	isLaneActive      []int
}

func readCases(t *testing.T) []testCase {
	js, err := os.ReadFile("test_vectors.json")
	assert.NoError(t, err)
	var cases []struct {
		In                []string `json:"in"`
		Hash              []string `json:"hash"`
		Lanes             []string `json:"lanes"`
		IsFirstLaneOfHash []int    `json:"IsFirstLaneOfHash"`
		IsLaneActive      []int    `json:"IsLaneActive"`
	}
	hsh := sha3.NewLegacyKeccak256()
	if err = json.Unmarshal(js, &cases); err != nil {
		panic(err)
	}
	res := make([]testCase, len(cases))
	for i, c := range cases {

		assert.Equal(t, len(c.Hash), len(c.In))

		res[i].in = make([][]byte, len(c.In))
		res[i].hash = make([][32]byte, len(c.Hash))

		for j := range c.Hash {

			b, err := utils.HexDecodeString(c.Hash[j]) // TODO find out how the hash results are represented in columns
			assert.NoError(t, err)
			assert.Equal(t, 32, len(b))

			copy(res[i].hash[j][:], b)

			res[i].in[j], err = utils.HexDecodeString(c.In[j])
			assert.NoError(t, err)

			hsh.Reset()
			hsh.Write(res[i].in[j])
			assert.Equal(t, utils.HexEncodeToString(hsh.Sum(nil)), c.Hash[j])
		}

		assert.Equal(t, len(res[i].lanes), len(res[i].isLaneActive))
		assert.Equal(t, len(res[i].lanes), len(res[i].isFirstLaneOfHash))
		res[i].isLaneActive = c.IsLaneActive
		res[i].isFirstLaneOfHash = c.IsFirstLaneOfHash
		res[i].lanes = make([][8]byte, len(c.Lanes))
		for j := range c.Lanes {
			b, err := utils.HexDecodeString(c.Lanes[j])
			assert.NoError(t, err)
			copy(res[i].lanes[j][len(res[i].lanes[j])-len(b):], b)
			assert.True(t, c.IsFirstLaneOfHash[j] == 0 || c.IsFirstLaneOfHash[j] == 1)
			assert.True(t, c.IsLaneActive[j] == 0 || c.IsLaneActive[j] == 1)
			// TODO check if firstLine => active?
			// TODO check that #(IsFirstLaneOfHash && active) == len(in)
		}
	}

	return res
}

func getTestCases(t *testing.T) []testCase {
	cases := sync.OnceValue(func() []testCase { return readCases(t) })()
	assert.NotEmpty(t, cases)
	return cases
}

func TestReadCases(t *testing.T) {
	getTestCases(t)
}

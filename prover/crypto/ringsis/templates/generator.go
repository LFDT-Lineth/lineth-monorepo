package main

import (
	"flag"
	"fmt"
	"math/big"
	"math/bits"
	"os"
	"text/template"

	"github.com/consensys/bavard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Config stores the template generation parameters for the optimized ring-SIS
type Config struct {
	ModulusDegree int64
	LogTwoBound   int64
}

func main() {

	cfg := Config{}
	flag.Int64Var(&cfg.LogTwoBound, "logTwoBound", 0, "")
	flag.Int64Var(&cfg.ModulusDegree, "modulusDegree", 0, "")
	flag.Parse()

	filesList := []string{
		"transversal_hash.go",
		"transversal_hash_test.go",
		"partial_fft.go",
		"twiddles.go",
		"partial_fft_test.go",
		"limb_decompose_test.go",
	}

	for _, file := range filesList {
		var (
			source = "./templates/" + file + ".tmpl"
			target = fmt.Sprintf("./ringsis_%v_%v/%v", cfg.ModulusDegree, cfg.LogTwoBound, file)
		)

		err := bavard.GenerateFromFiles(
			target,
			[]string{source},
			cfg,
			bavard.Funcs(template.FuncMap{
				"partialFFT": partialFFT,
				"pow":        pow,
				"bitReverse": bitReverse,
				"log2":       log2,
			}),
		)

		if err != nil {
			fmt.Printf("err = %v\n", err.Error())
			os.Exit(1)
		}
	}
}

func pow(base, pow int64) int64 {
	var (
		b = new(big.Int).SetInt64(base)
		p = new(big.Int).SetInt64(pow)
	)
	b.Exp(b, p, nil)

	if !b.IsInt64() {
		utils.Panic("could not cast big.Int %v to int64 as it overflows", b.String())
	}

	return b.Int64()
}

func log2(n int64) int64 {
	return int64(utils.Log2Floor(int(n)))
}

func bitReverse(n, i int64) uint64 {
	nn := uint64(64 - bits.TrailingZeros64(uint64(n)))
	r := make([]uint64, n)
	for i := 0; i < len(r); i++ {
		r[i] = uint64(i)
	}
	for i := 0; i < len(r); i++ {
		irev := bits.Reverse64(r[i]) >> nn
		if irev > uint64(i) {
			r[i], r[irev] = r[irev], r[i]
		}
	}
	return r[i]
}

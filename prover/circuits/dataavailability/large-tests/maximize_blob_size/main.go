package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/profile"
	"github.com/consensys/linea-monorepo/prover/circuits/dataavailability/config"
	v2 "github.com/consensys/linea-monorepo/prover/circuits/dataavailability/v2"
	"github.com/consensys/linea-monorepo/prover/circuits/execution"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

const (
	maxNbBatches = 100
	dictNbBytes  = 65536
)

func nbConstraints(maxUncompressedNbBytes int) int {
	fmt.Printf("*********************\nfor blob of size %d B or %.2fKB:\n", maxUncompressedNbBytes, float32(maxUncompressedNbBytes)/1024)
	c := v2.Circuit{
		BlobBytes: make([]frontend.Variable, 32*4096),
		Dict:      make([]frontend.Variable, dictNbBytes),
		FuncPI: v2.FunctionalPublicInputSnark{
			BatchSums: make([]execution.DataChecksumSnark, maxNbBatches),
		},
		CircuitSizes: config.CircuitSizes{
			MaxUncompressedNbBytes: maxUncompressedNbBytes,
			MaxNbBatches:           maxNbBatches,
			DictNbBytes:            dictNbBytes,
		},
	}
	runtime.GC()

	if *flagProfile {
		p := profile.Start(profile.WithPath(fmt.Sprintf("da-circuit-%sK.pprof", formatFloat(float64(maxUncompressedNbBytes)/1024.0))))
		defer p.Stop()
	}

	if cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &c, frontend.WithCapacity(*flagTargetNbConstraints*6/5)); err != nil {
		panic(err)
	} else {
		res := cs.GetNbConstraints()
		cmp := "match"
		if res > *flagTargetNbConstraints {
			cmp = "over"
		}
		if res < *flagTargetNbConstraints {
			cmp = "under"
		}
		fmt.Printf("%d constraints (%s)\n", res, cmp)
		return res
	}
}

var (
	flagCrawlStep           = flag.Int("step", 1000, "the crawl step") // TODO @Tabaie fix mixed metaphor
	flagStart               = flag.Int("start", blob.MaxUncompressedBytes, "initial size in bytes")
	flagTargetNbConstraints = flag.Int("target", 1<<27, "target number of constraints")
	flagBound1              = flag.Int("bound1", -1, "last size")
	flagBound2              = flag.Int("bound2", -1, "second to last size")
	flagProfile             = flag.Bool("profile", false, "enable profiling")
)

func main() {

	flag.Parse()

	var a, b int // lower and upper bounds

	// if given bounds, start the binary search
	if *flagBound1 != -1 && *flagBound2 != -1 {
		a, b = *flagBound1, *flagBound2
		if a > b {
			a, b = b, a
		}
		fmt.Print("bounds given.")
	} else { // only one value given, start crawling
		v := nbConstraints(*flagStart)
		a, b = *flagStart, *flagStart

		if v > *flagTargetNbConstraints {
			fmt.Println("crawling downward")
			for v > *flagTargetNbConstraints {
				b = a
				a = max(a-*flagCrawlStep, 0)
				v = nbConstraints(a)
				*flagCrawlStep *= 2
			}
		} else if v < *flagTargetNbConstraints {
			fmt.Println("crawling upward")
			for v < *flagTargetNbConstraints {
				a = b
				b += *flagCrawlStep
				v = nbConstraints(b)
				*flagCrawlStep *= 2
			}
		}
		if v == *flagTargetNbConstraints {
			fmt.Println("wow what are the odds")
			return
		}
		fmt.Print("bounds found.")
	}

	fmt.Println(" binary searching")

	for b > a {
		m := (b + a) / 2
		v := nbConstraints(m)
		if v > *flagTargetNbConstraints {
			b = m
		}
		if v < *flagTargetNbConstraints {
			a = v
		}
		if v == *flagTargetNbConstraints {
			return
		}
	}
}

func formatFloat(f float64) string {
	res := fmt.Sprintf("%f", f)
	if !strings.Contains(res, ".") {
		return res
	}
	for res[len(res)-1] == '0' {
		res = res[:len(res)-1]
	}
	if res[len(res)-1] == '.' {
		res = res[:len(res)-1]
	}
	return res
}

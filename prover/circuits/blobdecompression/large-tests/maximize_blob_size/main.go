package main

import (
	"flag"
	"fmt"
	"runtime"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	v1 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

func nbConstraints(blobSize int) int {
	fmt.Printf("*********************\nfor blob of size %d B or %.2fKB:\n", blobSize, float32(blobSize)/1024)
	c := v1.Circuit{
		BlobBytes:             make([]frontend.Variable, 32*4096),
		Dict:                  make([]frontend.Variable, 64*1024),
		MaxBlobPayloadNbBytes: blobSize,
		UseGkrMiMC:            true,
	}
	runtime.GC()
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

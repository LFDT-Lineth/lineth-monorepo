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
	fmt.Printf("*********************\nfor blob of size %dB or %.2fKB:\n", blobSize, float32(blobSize)/1024)
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
)

func main() {

	flag.Parse()
	v := nbConstraints(*flagStart)
	a, b := *flagStart, *flagStart

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
	fmt.Println("bounds found. binary searching")

	for b > a {
		m := (b + a) / 2
		v = nbConstraints(m)
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

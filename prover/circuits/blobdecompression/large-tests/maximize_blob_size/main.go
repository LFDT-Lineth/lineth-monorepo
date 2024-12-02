package main

import (
	"flag"
	"fmt"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	v1 "github.com/consensys/linea-monorepo/prover/circuits/blobdecompression/v1"
	blob "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
	"runtime"
)

const maxNbConstraints = 1 << 27

func nbConstraints(blobSize int) int {
	fmt.Printf("*********************\nfor blob of size %dB or %.2fKB:\n", blobSize, float32(blobSize)/1024)
	c := v1.Circuit{
		BlobBytes:             make([]frontend.Variable, 32*4096),
		Dict:                  make([]frontend.Variable, 64*1024),
		MaxBlobPayloadNbBytes: blobSize,
		UseGkrMiMC:            true,
	}
	runtime.GC()
	if cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &c, frontend.WithCapacity(maxNbConstraints*6/5)); err != nil {
		panic(err)
	} else {
		res := cs.GetNbConstraints()
		cmp := "match"
		if res > maxNbConstraints {
			cmp = "over"
		}
		if res < maxNbConstraints {
			cmp = "under"
		}
		fmt.Printf("%d constraints (%s)\n", res, cmp)
		return res
	}
}

var (
	flagCrawlStep = flag.Int("step", 1000, "the crawl step") // TODO @Tabaie fix mixed metaphor
	flagStart     = flag.Int("start", blob.MaxUncompressedBytes, "initial size in bytes")
)

func main() {

	flag.Parse()
	v := nbConstraints(*flagStart)
	a, b := *flagStart, *flagStart

	if v > maxNbConstraints {
		fmt.Println("crawling downward")
		for v > maxNbConstraints {
			b = a
			a = max(a-*flagCrawlStep, 0)
			v = nbConstraints(a)
			*flagCrawlStep *= 2
		}
	} else if v < maxNbConstraints {
		fmt.Println("crawling upward")
		for v < maxNbConstraints {
			a = b
			b += *flagCrawlStep
			v = nbConstraints(b)
			*flagCrawlStep *= 2
		}
	}
	if v == maxNbConstraints {
		fmt.Println("wow what are the odds")
		return
	}
	fmt.Println("bounds found. binary searching")

	for b > a {
		m := (b + a) / 2
		v = nbConstraints(m)
		if v > maxNbConstraints {
			b = m
		}
		if v < maxNbConstraints {
			a = v
		}
		if v == maxNbConstraints {
			return
		}
	}
}

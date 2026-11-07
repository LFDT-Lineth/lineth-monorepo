package predecoding

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding/zkccheck"
)

// Runs the ZkC predecoding checker (generated Go) on every word in
// [ZKC_START, ZKC_END): it must accept the Go decoder's record, and must
// reject mutated records where the mutated field matters.
func TestZkcCheckerAgainstGo(t *testing.T) {
	start, _ := strconv.ParseUint(os.Getenv("ZKC_START"), 0, 64)
	end, _ := strconv.ParseUint(os.Getenv("ZKC_END"), 0, 64)
	if end == 0 {
		end = start + 1<<20
	}
	cats := map[string]uint64{}
	samples := map[string]string{}
	note := func(k, s string) {
		cats[k]++
		if _, ok := samples[k]; !ok {
			samples[k] = s
		}
	}
	var bw bitWriter
	t0 := time.Now()
	for i := start; i < end; i++ {
		x := uint32(i)
		bw = bitWriter{buf: bw.buf[:0]}
		decodeInstruction(&bw, x)
		g := readRecord(bw.buf)
		r := [5]uint64{g.op, g.imm, g.rs1, g.rs2, g.rd}
		if err := zkccheck.CheckWord(x, r); err != nil {
			note("rejects-go-record", fmt.Sprintf("%#08x op=%d err=%v", x, g.op, err))
			continue
		}
		muts := []struct {
			name string
			r    [5]uint64
		}{
			{"op", [5]uint64{(g.op + 1) % 256, g.imm, g.rs1, g.rs2, g.rd}},
			{"imm", [5]uint64{g.op, g.imm ^ 1, g.rs1, g.rs2, g.rd}},
			{"rs1", [5]uint64{g.op, g.imm, g.rs1 ^ 1, g.rs2, g.rd}},
			{"rs2", [5]uint64{g.op, g.imm, g.rs1, g.rs2 ^ 1, g.rd}},
			{"rd", [5]uint64{g.op, g.imm, g.rs1, g.rs2, g.rd ^ 1}},
		}
		for _, m := range muts {
			if zkccheck.CheckWord(x, m.r) == nil {
				note(fmt.Sprintf("accepts-mutated-%s/op=%d", m.name, g.op), fmt.Sprintf("%#08x", x))
			}
		}
	}
	keys := make([]string, 0, len(cats))
	for k := range cats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("CAT %s count=%d e.g. %s\n", k, cats[k], samples[k])
	}
	fmt.Printf("RANGE %#x-%#x words=%d elapsed=%s\n", start, end, end-start, time.Since(t0))
}

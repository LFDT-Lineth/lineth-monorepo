package zkccheck

// CheckWord runs the generated ZkC predecoding check on one raw word against
// one decoded record (compute_op, imm, rs1, rs2, rd). Not goroutine-safe: the
// generated code uses package-level input memories.
func CheckWord(word uint32, rec [5]uint64) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if f, ok := r.(failure); ok {
				err = f
				return
			}
			panic(r)
		}
	}()
	mem_instruction_base = []uint64{0}
	mem_decoded = rec[:]
	fn_predecoding(0, uint64(word))
	return nil
}

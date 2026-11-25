# Word-wise memcpy / memmove / memset for the rv64im guests.
#
# Why: guests are built ReleaseSmall, where Zig's compiler_rt exports byte-at-a-time mem*
# (memcpySmall & co., ~6 instructions per byte). Guest cost is executed instructions, and these
# loops were up to ~40% of l2-execution and >50% of some Constantine precompiles. compiler_rt's
# mem* are weak, so these strong definitions replace them at link time.
#
# Written in asm so the compiler cannot turn a copy loop back into a call to itself.
#
# Shape: byte head until dst is 8-aligned, 4x8-byte unrolled loop, 8-byte loop, byte tail. Stores
# are always aligned; loads may be misaligned (the target profile includes Zicclsm, which the ZkC
# interpreter implements), so mismatched src/dst alignments take the same word path.

	.section .text.memcpy,"ax",@progbits

# void *memmove(void *dst, const void *src, size_t n)
	.globl memmove
	.type memmove,@function
memmove:
	mv t6, a0
	sub t0, a0, a1
	bgeu t0, a2, .Lcpy_head          # dst - src >= n (unsigned): forward copy is safe
	# dst overlaps the tail of src: copy backward from the ends.
	add t6, a0, a2
	add a1, a1, a2
.Lmov_head:                          # byte tail until the dst end is 8-aligned
	andi t0, t6, 7
	beqz t0, .Lmov_aligned
	beqz a2, .Lret
	addi a1, a1, -1
	addi t6, t6, -1
	lbu t1, 0(a1)
	sb t1, 0(t6)
	addi a2, a2, -1
	j .Lmov_head
.Lmov_aligned:
	andi t0, a2, -32
	beqz t0, .Lmov_words
	sub t0, t6, t0
.Lmov_blk:                           # all loads before stores: overlap-safe
	ld t1, -8(a1)
	ld t2, -16(a1)
	ld t3, -24(a1)
	ld t4, -32(a1)
	sd t1, -8(t6)
	sd t2, -16(t6)
	sd t3, -24(t6)
	sd t4, -32(t6)
	addi a1, a1, -32
	addi t6, t6, -32
	bne t6, t0, .Lmov_blk
	andi a2, a2, 31
.Lmov_words:
	andi t0, a2, -8
	beqz t0, .Lmov_bytes
	sub t0, t6, t0
.Lmov_w:
	ld t1, -8(a1)
	sd t1, -8(t6)
	addi a1, a1, -8
	addi t6, t6, -8
	bne t6, t0, .Lmov_w
	andi a2, a2, 7
.Lmov_bytes:
	beqz a2, .Lret
	sub t0, t6, a2
.Lmov_b:
	lbu t1, -1(a1)
	sb t1, -1(t6)
	addi a1, a1, -1
	addi t6, t6, -1
	bne t6, t0, .Lmov_b
	ret
	.size memmove, .-memmove

# void *memcpy(void *dst, const void *src, size_t n)
# Also memmove's forward path: each step loads before it stores, so dst < src overlap is safe.
	.globl memcpy
	.type memcpy,@function
memcpy:
	mv t6, a0
.Lcpy_head:                          # byte head until dst is 8-aligned
	andi t0, t6, 7
	beqz t0, .Lcpy_aligned
	beqz a2, .Lret
	lbu t1, 0(a1)
	sb t1, 0(t6)
	addi a1, a1, 1
	addi t6, t6, 1
	addi a2, a2, -1
	j .Lcpy_head
.Lcpy_aligned:
	andi t0, a2, -32
	beqz t0, .Lcpy_words
	add t0, t6, t0
.Lcpy_blk:
	ld t1, 0(a1)
	ld t2, 8(a1)
	ld t3, 16(a1)
	ld t4, 24(a1)
	sd t1, 0(t6)
	sd t2, 8(t6)
	sd t3, 16(t6)
	sd t4, 24(t6)
	addi a1, a1, 32
	addi t6, t6, 32
	bne t6, t0, .Lcpy_blk
	andi a2, a2, 31
.Lcpy_words:
	andi t0, a2, -8
	beqz t0, .Lcpy_bytes
	add t0, t6, t0
.Lcpy_w:
	ld t1, 0(a1)
	sd t1, 0(t6)
	addi a1, a1, 8
	addi t6, t6, 8
	bne t6, t0, .Lcpy_w
	andi a2, a2, 7
.Lcpy_bytes:
	beqz a2, .Lret
	add t0, t6, a2
.Lcpy_b:
	lbu t1, 0(a1)
	sb t1, 0(t6)
	addi a1, a1, 1
	addi t6, t6, 1
	bne t6, t0, .Lcpy_b
.Lret:
	ret
	.size memcpy, .-memcpy

	.section .text.memset,"ax",@progbits

# void *memset(void *dst, int c, size_t n)
	.globl memset
	.type memset,@function
memset:
	mv t6, a0
	andi a1, a1, 255
	beqz a1, .Lset_head              # zero fill (the common case) needs no splat
	slli t0, a1, 8                   # splat the byte into all 8 lanes
	or a1, a1, t0
	slli t0, a1, 16
	or a1, a1, t0
	slli t0, a1, 32
	or a1, a1, t0
.Lset_head:                          # byte head until dst is 8-aligned
	andi t0, t6, 7
	beqz t0, .Lset_aligned
	beqz a2, .Lset_ret
	sb a1, 0(t6)
	addi t6, t6, 1
	addi a2, a2, -1
	j .Lset_head
.Lset_aligned:
	andi t0, a2, -32
	beqz t0, .Lset_words
	add t0, t6, t0
.Lset_blk:
	sd a1, 0(t6)
	sd a1, 8(t6)
	sd a1, 16(t6)
	sd a1, 24(t6)
	addi t6, t6, 32
	bne t6, t0, .Lset_blk
	andi a2, a2, 31
.Lset_words:
	andi t0, a2, -8
	beqz t0, .Lset_bytes
	add t0, t6, t0
.Lset_w:
	sd a1, 0(t6)
	addi t6, t6, 8
	bne t6, t0, .Lset_w
	andi a2, a2, 7
.Lset_bytes:
	beqz a2, .Lset_ret
	add t0, t6, a2
.Lset_b:
	sb a1, 0(t6)
	addi t6, t6, 1
	bne t6, t0, .Lset_b
.Lset_ret:
	ret
	.size memset, .-memset

	.attribute	4, 16
	.attribute	5, "rv64i2p1_m2p0_zmmul1p0"
	.file	"opcode-cost"
	.text
	.p2align	2
	.type	.Lmain.r5_main,@function
.Lmain.r5_main:
	.cfi_startproc
	addi	sp, sp, -32
	.cfi_def_cfa_offset 32
	sd	ra, 24(sp)
	sd	s0, 16(sp)
	sd	s1, 8(sp)
	.cfi_offset ra, -8
	.cfi_offset s0, -16
	.cfi_offset s1, -24
	lui	a0, 74565
	lui	a1, 30292
	addi	a0, a0, 1656
	addi	a1, a1, 801
	sw	a0, 0(sp)
	sw	a1, 4(sp)
	lwu	s0, 0(sp)
	lwu	s1, 4(sp)
	li	a0, 10
	li	a1, 0
	call	.Lmain.emitMark
	addi	a1, s0, 1000
	li	a0, 11
	call	.Lmain.emitMark
	li	a0, 20
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_2
.LBB0_1:
	remuw	a1, a1, s1
	slli	a1, a1, 32
	srli	a1, a1, 32
	addi	a0, a0, 1
	bnez	a0, .LBB0_1
.LBB0_2:
	li	a0, 21
	call	.Lmain.emitMark
	li	a0, 30
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_4
.LBB0_3:
	mul	a1, a1, s1
	addi	a0, a0, 1
	bnez	a0, .LBB0_3
.LBB0_4:
	li	a0, 31
	call	.Lmain.emitMark
	li	a0, 40
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_6
.LBB0_5:
	remu	a1, a1, s1
	addi	a0, a0, 1
	bnez	a0, .LBB0_5
.LBB0_6:
	li	a0, 41
	call	.Lmain.emitMark
	li	a0, 50
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, 1000
	mul	a1, s1, a0
	add	a1, a1, s0
	li	a0, 51
	call	.Lmain.emitMark
	#APP
	li	a0, 0
	li	a7, 93
	ecall
	#NO_APP
.Lfunc_end0:
	.size	.Lmain.r5_main, .Lfunc_end0-.Lmain.r5_main
	.cfi_endproc

	.section	.rodata.cst8,"aM",@progbits,8
	.p2align	3, 0x0
.LCPI1_0:
	.quad	5561177274562334799
	.text
	.p2align	2
	.type	.Lmain.emitMark,@function
.Lmain.emitMark:
	.cfi_startproc
	addi	sp, sp, -96
	.cfi_def_cfa_offset 96
	sd	ra, 88(sp)
	sd	s0, 80(sp)
	sd	s1, 72(sp)
	sd	s2, 64(sp)
	.cfi_offset ra, -8
	.cfi_offset s0, -16
	.cfi_offset s1, -24
	.cfi_offset s2, -32
	mv	s0, a1
	mv	a1, a0
	lui	a0, %hi(.LCPI1_0)
	ld	a0, %lo(.LCPI1_0)(a0)
	lui	a2, 38069
	addi	a2, a2, 577
	sd	a0, 0(sp)
	sw	a2, 8(sp)
	addi	a0, sp, 12
	call	.Lmain.decimalBuf
	mv	s1, sp
	li	a1, 9
	addi	s2, a0, 13
	add	a0, s1, a0
	sb	a1, 12(a0)
	add	a0, s1, s2
	mv	a1, s0
	call	.Lmain.decimalBuf
	add	a0, a0, s2
	li	a1, 10
	add	a2, s1, a0
	sb	a1, 0(a2)
	addi	a3, a0, 1
	#APP
	li	a0, 1
	mv	a1, s1
	mv	a2, a3
	li	a7, 64
	ecall
	#NO_APP
	ld	ra, 88(sp)
	ld	s0, 80(sp)
	ld	s1, 72(sp)
	ld	s2, 64(sp)
	.cfi_restore ra
	.cfi_restore s0
	.cfi_restore s1
	.cfi_restore s2
	addi	sp, sp, 96
	.cfi_def_cfa_offset 0
	ret
.Lfunc_end1:
	.size	.Lmain.emitMark, .Lfunc_end1-.Lmain.emitMark
	.cfi_endproc

	.p2align	2
	.type	.Lmain.decimalBuf,@function
.Lmain.decimalBuf:
	.cfi_startproc
	addi	sp, sp, -48
	.cfi_def_cfa_offset 48
	sd	ra, 40(sp)
	sd	s0, 32(sp)
	.cfi_offset ra, -8
	.cfi_offset s0, -16
	beqz	a1, .LBB2_4
	li	s0, 0
	addi	a2, sp, 32
	li	a3, 10
	li	a4, 246
	beqz	a1, .LBB2_3
.LBB2_2:
	divu	a5, a1, a3
	mul	a6, a5, a4
	add	a1, a6, a1
	ori	a1, a1, 48
	sb	a1, -1(a2)
	addi	a2, a2, -1
	addi	s0, s0, 1
	mv	a1, a5
	bnez	a5, .LBB2_2
.LBB2_3:
	mv	a1, a2
	mv	a2, s0
	call	memcpy
	j	.LBB2_5
.LBB2_4:
	li	a1, 48
	sb	a1, 0(a0)
	li	s0, 1
.LBB2_5:
	mv	a0, s0
	ld	ra, 40(sp)
	ld	s0, 32(sp)
	.cfi_restore ra
	.cfi_restore s0
	addi	sp, sp, 48
	.cfi_def_cfa_offset 0
	ret
.Lfunc_end2:
	.size	.Lmain.decimalBuf, .Lfunc_end2-.Lmain.decimalBuf
	.cfi_endproc

	.type	.L__anon_874,@object
	.section	.rodata.str1.1,"aMS",@progbits,1
.L__anon_874:
	.asciz	"OPCODE-MARK\t"
	.size	.L__anon_874, 13

	.globl	r5_main
	.type	r5_main,@function
r5_main = .Lmain.r5_main
	.section	".note.GNU-stack","",@progbits

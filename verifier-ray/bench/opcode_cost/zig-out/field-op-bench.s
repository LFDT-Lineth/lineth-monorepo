	.attribute	4, 16
	.attribute	5, "rv64i2p1_m2p0_zmmul1p0"
	.file	"field-op-bench"
	.text
	.p2align	2
	.type	.Lmain.r5_main,@function
.Lmain.r5_main:
	.cfi_startproc
	addi	sp, sp, -48
	.cfi_def_cfa_offset 48
	sd	ra, 40(sp)
	sd	s0, 32(sp)
	sd	s1, 24(sp)
	sd	s2, 16(sp)
	.cfi_offset ra, -8
	.cfi_offset s0, -16
	.cfi_offset s1, -24
	.cfi_offset s2, -32
	lui	a0, 74565
	lui	a1, 633806
	addi	a0, a0, 1656
	addi	a1, a1, -272
	sw	a0, 8(sp)
	sw	a1, 12(sp)
	lw	a0, 8(sp)
	lw	a1, 12(sp)
	lui	s2, 520192
	addi	s2, s2, 1
	remuw	s0, a0, s2
	remuw	s1, a1, s2
	li	a0, 10
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_2
.LBB0_1:
	add	a1, a1, s1
	remuw	a1, a1, s2
	addi	a0, a0, 1
	bnez	a0, .LBB0_1
.LBB0_2:
	lui	s2, 520192
	addi	s2, s2, 1
	li	a0, 11
	call	.Lmain.emitMark
	li	a0, 20
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_4
.LBB0_3:
	subw	a1, a1, s1
	add	a1, a1, s2
	remuw	a1, a1, s2
	addi	a0, a0, 1
	bnez	a0, .LBB0_3
.LBB0_4:
	li	a0, 21
	call	.Lmain.emitMark
	li	a0, 30
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	mv	a1, s0
	beqz	a0, .LBB0_6
.LBB0_5:
	sext.w	a2, a1
	subw	a1, s2, a1
	seqz	a2, a2
	addi	a2, a2, -1
	and	a1, a2, a1
	addi	a0, a0, 1
	bnez	a0, .LBB0_5
.LBB0_6:
	li	a0, 31
	call	.Lmain.emitMark
	li	a0, 40
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	lui	a2, 520192
	addi	a2, a2, 1
	mv	a1, s0
	beqz	a0, .LBB0_8
.LBB0_7:
	slli	a1, a1, 1
	remuw	a1, a1, a2
	addi	a0, a0, 1
	bnez	a0, .LBB0_7
.LBB0_8:
	li	a0, 41
	call	.Lmain.emitMark
	li	a0, 50
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	lui	a2, 520192
	addi	a2, a2, 1
	mv	a1, s0
	beqz	a0, .LBB0_10
.LBB0_9:
	sext.w	a1, a1
	mul	a1, a1, s1
	remu	a1, a1, a2
	addi	a0, a0, 1
	bnez	a0, .LBB0_9
.LBB0_10:
	li	a0, 51
	call	.Lmain.emitMark
	li	a0, 60
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, -1000
	lui	a2, 520192
	addi	a2, a2, 1
	mv	a1, s0
	beqz	a0, .LBB0_12
.LBB0_11:
	sext.w	a1, a1
	mul	a1, a1, a1
	remu	a1, a1, a2
	addi	a0, a0, 1
	bnez	a0, .LBB0_11
.LBB0_12:
	li	a0, 61
	call	.Lmain.emitMark
	li	a0, 70
	li	a1, 0
	call	.Lmain.emitMark
	li	a0, 0
	li	a1, 1000
	lui	a2, 520192
	addi	a2, a2, 1
	beqz	a1, .LBB0_18
.LBB0_13:
	li	a3, 1
	li	a4, 7
	beqz	a4, .LBB0_17
.LBB0_14:
	andi	a5, a4, 1
	sext.w	s0, s0
	beqz	a5, .LBB0_16
	sext.w	a3, a3
	mul	a3, a3, s0
	remu	a3, a3, a2
.LBB0_16:
	mul	a5, s0, s0
	remu	s0, a5, a2
	srliw	a4, a4, 1
	bnez	a4, .LBB0_14
.LBB0_17:
	addi	a0, a0, 1
	mv	s0, a3
	bne	a0, a1, .LBB0_13
.LBB0_18:
	li	a0, 71
	mv	a1, s0
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
	.quad	4705466957032671558
	.text
	.p2align	2
	.type	.Lmain.emitMark,@function
.Lmain.emitMark:
	.cfi_startproc
	addi	sp, sp, -112
	.cfi_def_cfa_offset 112
	sd	ra, 104(sp)
	sd	s0, 96(sp)
	sd	s1, 88(sp)
	sd	s2, 80(sp)
	sd	s3, 72(sp)
	.cfi_offset ra, -8
	.cfi_offset s0, -16
	.cfi_offset s1, -24
	.cfi_offset s2, -32
	.cfi_offset s3, -40
	mv	s0, a1
	mv	a1, a0
	lui	a0, %hi(.LCPI1_0)
	ld	a0, %lo(.LCPI1_0)(a0)
	li	s1, 9
	lui	a2, 5
	addi	a2, a2, -1198
	sd	a0, 8(sp)
	sh	a2, 16(sp)
	sb	s1, 18(sp)
	addi	a0, sp, 19
	call	.Lmain.decimalBuf
	addi	s2, sp, 8
	addi	s3, a0, 12
	add	a0, s2, a0
	sb	s1, 11(a0)
	add	a0, s2, s3
	sext.w	a1, s0
	call	.Lmain.decimalBuf
	add	a0, a0, s3
	li	a1, 10
	add	a2, s2, a0
	sb	a1, 0(a2)
	addi	a3, a0, 1
	#APP
	li	a0, 1
	mv	a1, s2
	mv	a2, a3
	li	a7, 64
	ecall
	#NO_APP
	ld	ra, 104(sp)
	ld	s0, 96(sp)
	ld	s1, 88(sp)
	ld	s2, 80(sp)
	ld	s3, 72(sp)
	.cfi_restore ra
	.cfi_restore s0
	.cfi_restore s1
	.cfi_restore s2
	.cfi_restore s3
	addi	sp, sp, 112
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

	.type	.L__anon_915,@object
	.section	.rodata.str1.1,"aMS",@progbits,1
.L__anon_915:
	.asciz	"FIELD-MARK\t"
	.size	.L__anon_915, 12

	.globl	r5_main
	.type	r5_main,@function
r5_main = .Lmain.r5_main
	.section	".note.GNU-stack","",@progbits

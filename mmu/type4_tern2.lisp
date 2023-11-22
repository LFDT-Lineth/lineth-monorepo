(module mmu)

; === PREPROCESSING ===

(defun (preprocessing-type4-tern2)
  (if-eq TERNARY tern2
    (begin
      (eq OFF_1_LO (+ (* 16 ACC_3) NIB_3))
      (eq (+ OFF_1_LO (- SIZE_IMPORTED 1)) (+ (* 16 ACC_4) NIB_4))

      (eq TOT (+ (- ACC_4 ACC_3) 1))

      (if-eq-else TOT 1
        (eq BIT_1 1)
        (vanishes! BIT_1)
      )

      (if-zero-else NIB_3
        (eq BIT_3 1)
        (vanishes! BIT_3)
      )

      (if-eq-else NIB_4 15
        (eq BIT_4 1)
        (vanishes! BIT_4)
      )

      (will-eq! TLO TLO)
      (will-eq! TBO TBO)
      (eq TLO ACC_3)
      (eq TBO NIB_3)
    )
  )
)

; === MICRO-WRITING ===

;4.9.2
(defun (micro-instruction-writing-type4-tern2)
(if-eq TERNARY tern2
  (if-eq IS_MICRO 1
    (begin
      ;1
      (eq TLO (+ (shift TLO -1) (shift IS_MICRO -1)))
      (if-zero-else BIT_1
        ;3
        (micro-instruction-type4-bit_1-is-not-set)
        ;2
        (micro-instruction-type4-bit_1-is-set)
      )
    )
  )
)
)

(defun (micro-instruction-type4-bit_1-is-set)
 (begin
  (if-zero-else BIT_3
    ;2.c
    (begin
      (eq SIZE SIZE_IMPORTED)
      (eq MICRO_INST RamLimbExcision)
    )
    (if-zero-else BIT_4
      ;2.c
      (begin
        (eq SIZE SIZE_IMPORTED)
        (eq MICRO_INST RamLimbExcision)
      )
      ;2.b
      (eq MICRO_INST KillingOne)
    )
  )
 )
)

;3
(defun (micro-instruction-type4-bit_1-is-not-set)
  (if-zero-else (shift MICRO_INST -1)
    ;3.d
    (begin
      (if-zero-else BIT_3
        ;3.d.ii
        (begin
          (eq SIZE (+ (- 15 NIB_1) 1))
          (eq MICRO_INST RamLimbExcision)
        )
        ;3.d.i
        (begin
          (eq MICRO_INST KillingOne)
        )
     )
    )
    ;3.e
    (begin
      ;3.e.i
      (vanishes! TBO)
      (if-zero-else TOT
        ;TOT=0
        ;3.e.iii & 3.e.iv
        (begin
          ;
          (if-zero-else BIT_4
            ;B
            (begin
              (eq SIZE (+ NIB_4 1))
              (eq MICRO_INST RamLimbExcision)
            )
            ;A
            (eq MICRO_INST KillingOne)
          )
        )
        ;TOT!=0
        ;3.e.ii
        (eq MICRO_INST KillingOne)
      )
    )
  )
)

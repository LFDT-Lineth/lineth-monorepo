(module mmu)

;====== precomputation ======

(defun (euclidian-division-type2)
  (begin
    ;OFF_1 = 16 * ACC_1 + NIB_1
    (eq OFF_1_LO
      (+
        (* 16 ACC_1)
        NIB_1
      )
    )

    ;OFF_1 + MIN -1 = 16 * ACC_2 + NIB_2
    (eq
      (+ OFF_1_LO (- MIN 1))
      (+
        (* 16 ACC_2)
        NIB_2
      )
    )

    ;R@O = 16 * ACC_3 + NIB_3
    (eq RETURN_OFFSET
      (+
        (* 16 ACC_3)
        NIB_3
      )
    )

    ;R@O + MIN - 1 = 16 * ACC_4 + NIB_4
    (eq
      (+ RETURN_OFFSET (- MIN 1))
      (+
        (* 16 ACC_4)
        NIB_4
      )
    )
  )
)

(defun (comparisions-type2)
  (begin
    ;ACC_5 = (R@C - SIZE) * (2 * BIT_1 -1) - BIT_1
    (eq
      ACC_5
       (- (*
          (- RETURN_CAPACITY SIZE_IMPORTED)
          (- (* 2 BIT_1) 1)
          )
        BIT_1
      )
    )
    ;NIB_5 = (NIB_3 - NIB_1) * (2 * BIT_2 - 1) - BIT_2
    (eq
      NIB_5
       (- (*
          (- NIB_3 NIB_1)
          (- (* 2 BIT_2) 1)
          )
        BIT_2
      )
    )
    ;NIB_6 = (NIB_2 - NIB 4) * (2 * BIT_3 - 1) - BIT_3
    (eq
      NIB_6
       (- (*
          (- NIB_2 NIB_4)
          (- (* 2 BIT_3) 1)
          )
        BIT_3
      )
    )
  )
)

(defun (min-type2)
  (eq MIN (+ (* BIT_1 SIZE_IMPORTED) (* (- 1 BIT_1) RETURN_CAPACITY) ))
)


(defun (workflow-parameters-type2)
  (begin
    (eq TOT (+ (- ACC_2 ACC_1) 1))

    (if-eq-else TOT 1
     (eq BIT_4 1)
     (vanishes! BIT_4)
    )

    (if-zero-else BIT_4
      (vanishes! BIT_5)
      ;NIB_7 = NIB_3 + MIN - 1 - 16 * BIT_5
      (eq NIB_7 (- (+ NIB_3 (- MIN 1)) (* 16 BIT_5)) )
    )

    (if-eq-else NIB_1 NIB_3
       (eq ALIGNED 1)
       (vanishes! ALIGNED)
    )
  )
)

(defun (limb-and-byte-offsets-type2)
    (begin
      (will-eq! SLO SLO)
      (eq SLO ACC_1)

      (will-eq! SBO SBO)
      (eq SBO NIB_1)

      (will-eq! TLO TLO)
      (eq TLO ACC_3)

      (will-eq! TBO TBO)
      (eq TBO NIB_3)
    )
)

(defun (preprocessing-type2)
  (if-zero IS_MICRO
    (if-eq (shift IS_MICRO 1) 1
      (begin
        (euclidian-division-type2)
        (comparisions-type2)
        (min-type2)
        (workflow-parameters-type2)
        (limb-and-byte-offsets-type2)
      )
    )
  )
)

; ====== micro-instruction-writing =====

(defun (micro-instruction-writing-bit-4-true-type2)
  (if-eq BIT_4 1
    (begin
      (eq SIZE MIN)
      (if-zero-else BIT_2
        (begin
          (eq MICRO_INST RamToRamSlideChunk)
          (vanishes! FAST)
        )
        (if-zero-else BIT_5
          (begin
            (eq MICRO_INST RamToRamSlideChunk)
            (vanishes! FAST)
          )
          (begin
            (eq MICRO_INST RamToRamSlideOverlappingChunk)
            (vanishes! FAST)
          )
        )
      )
    )
  )
)

(defun (micro-instruction-writing-bit-4-false-type2)
  (if-zero BIT_4
    (begin
      ;a & c
      (if-zero (shift IS_MICRO -1)
        (begin
          ;a
          (will-eq! TLO (+ TLO (+ ALIGNED BIT_2)))
          ;c
          (eq SIZE (+ (- 15 NIB_1) 1))
          (if-zero-else BIT_2
            (begin
              (eq MICRO_INST RamToRamSlideChunk)
              (vanishes! FAST)
            )
            (begin
              (eq MICRO_INST RamToRamSlideOverlappingChunk)
              (vanishes! FAST)
            )
          )
        )
      )

      (if-eq (shift IS_MICRO -1) 1
        (begin
          ;b
          (if-eq IS_MICRO 1
            (if-eq (shift IS_MICRO 1) 1
              (will-eq! TLO (+ TLO 1))
            )
          )
          ;d
          ;i
          (vanishes! SBO)
          ;ii TBO = NIB_3 + 16 - NIB_1 - 16 * (ALIGNED + BIT_2)
          (eq TBO
            (-
              (- (+ NIB_3 16)
                 NIB_1
              )
              (* 16 (+ ALIGNED BIT_2) )
            )
          )

          (if-zero-else TOT
          ;iv
            (begin
              (eq SIZE (+ NIB_2 1))
              (if-zero-else BIT_3
                (begin
                  (eq MICRO_INST RamToRamSlideChunk)
                  (vanishes! FAST)
                )
                (begin

                  (eq MICRO_INST RamToRamSlideOverlappingChunk)
                  (vanishes! FAST)
                )
              )
            )
          ;iii
            (begin
              (eq SIZE 16)
              (if-zero-else ALIGNED
                (begin
                  (eq MICRO_INST RamToRamSlideOverlappingChunk)
                  (vanishes! FAST)
                )
                (begin
                  (eq MICRO_INST RamToRam)
                  (eq FAST 1)
                )
              )
            )
          )
        )
      )
    )
  )
)

(defun (micro-instruction-writing-type2)
  (if-eq IS_MICRO 1
  (begin
    ;1.
    (eq SLO (+ (shift SLO -1) (shift IS_MICRO -1)))
    ;2
    (micro-instruction-writing-bit-4-true-type2)
    ;3
    (micro-instruction-writing-bit-4-false-type2)
  ))
)

;====== type2 ======

(defconstraint type2 ()
  (if-eq PRE type2
    (begin
      (eq CN_S CN)
      (eq CN_T CALLER)
      (vanishes! OFFOOB)
      (preprocessing-type2)
      (micro-instruction-writing-type2)
    )
  )
)

(module mmu)

;====== precomputation ======

(defun (context-and-offoob-type5)
  (begin
    (vanishes! OFFOOB)
    (if-eq-else CSD 1
      (vanishes! CN_S)
      (eq CN_S CALLER)
    )
  )
)

(defun (max-offset-type5)
  (begin
    (eq ACC_1
        (+ (* ( - (* 2 BIT_1) 1)
              (- CDS (+ OFF_1_LO 32))
           )
           (- BIT_1 1)
        )
    )
    (eq (+ ACC_2 1)
      (+ (* 32 BIT_1)
         (* (- 1 BIT_1)
           (- CDS OFF_1_LO)
         )
      )
    )
    (eq ACC_2 (+ (* 16 BIT_2) NIB_2))
    (if-eq-else NIB_2 15
       (eq BIT_3 1)
       (vanishes! BIT_3)
    )
  )
)
(defun (tot-type5)
  (if-eq-else CSD 1
    (eq TOT 4)
    (eq TOT 1)
  )
)

(defun (aligned-type5)
  (begin
    (eq (+ CDO OFF_1_LO) (+ (* 16 ACC_3) NIB_3))
    (if-zero-else NIB_3
      (eq ALIGNED 1)
      (vanishes! ALIGNED)
    )
  )
)

(defun (bit4-type5)
  (eq NIB_4
    (- (* (- (* 2 BIT_4) 1)
          (- (+ NIB_2 1) (+(- 15 NIB_3) 1))
       )
       BIT_4
    )
  )
)

(defun (offsets-type5)
  (begin
    (eq SLO ACC_3)
    (will-eq! SLO SLO)

    (eq SBO NIB_3)
    (will-eq! SBO SBO)

    (vanishes! TLO)
    (will-eq! TLO TLO)

    (vanishes! TBO)
    (will-eq! TBO TBO)
  )
)

(defun (preprocessing-type5)
  (if-zero IS_MICRO
    (if-eq (shift IS_MICRO 1) 1
      (begin
       (context-and-offoob-type5)
       (max-offset-type5)
       (tot-type5)
       (aligned-type5)
       (bit4-type5)
       (offsets-type5)
      )
    )
  )
)

;====
(defun (csd-eq-one-type5)
    (if-eq CSD 1
      (begin
        (if-eq (shift IS_MICRO -1) 0
          (begin
            (eq MICRO_INST StoreXInAThreeRequired)
            (eq (shift MICRO_INST 1) StoreXInB)
            (eq (shift MICRO_INST 2) StoreXInC)
          )
        )
        (if-zero-else TOT
          (begin
            (vanishes! SLO)
            (eq SBO (shift SBO -1))
            (vanishes! ERF)
            (vanishes! EXO_IS_TXCD)
            (eq SIZE (+ NIB_2 1))
            (eq FAST (* ALIGNED BIT_3))
          )
          (begin
            (eq SLO (+ (shift SLO -1) (shift IS_MICRO -1)))
            (eq SBO (shift SBO -1))
            (eq ERF 1)
            (eq EXO_IS_TXCD 1)
            (eq FAST 1)
          )
        )
      )
    )
)



(defun (micro-instruction-writing-aligned)
  (begin
    (if-zero-else BIT_2
      (if-zero-else BIT_3
        ;[2] == 0 && [3] == 0
        (begin
          (eq MICRO_INST FirstPaddedSecondZero)
          ; SIZE already set in csd-eq-one-type5
          ;(eq SIZE (+ 1 NIB_2))
        )
        ;[2] == 0 && [3] == 1
        (eq MICRO_INST PushOneRamToStack)
      )

      (if-zero-else BIT_3
        ;[2] == 1 && [3] == 0
        (begin
          (eq MICRO_INST FirstFastSecondPadded)
          ; SIZE already set in csd-eq-one-type5
          ;(eq SIZE (+ 1 NIB_2))
        )
        ;[2] == 1 && [3] == 1
        (eq MICRO_INST PushTwoRamToStack)
      )
    )
  )
)

(defun (micro-instruction-writing-not-aligned)
  (begin
    (if-zero-else BIT_2
      (if-zero-else BIT_3
        (begin
          (if-zero-else BIT_4
            ;[2] == 0 && [3] == 0 && [4] == 0
            (eq MICRO_INST NA_RamToStack_1To1PaddedAndZero)
            ;[2] == 0 && [3] == 0 && [4] == 1
            (eq MICRO_INST NA_RamToStack_2To1PaddedAndZero)
          )
          ; SIZE already set in csd-eq-one-type5
          ;(eq SIZE (+ 1 NIB_2))
        )
        ;[2] == 0 && [3] == 1
        (eq MICRO_INST NA_RamToStack_2To1FullAndZero)
      )

      (if-zero-else BIT_3
        (begin
          (if-zero-else BIT_4
            ;[2] == 1 && [3] == 0 && [4] == 0
            (eq MICRO_INST NA_RamToStack_2To2Padded)
            ;[2] == 1 && [3] == 0 && [4] == 1
            (eq MICRO_INST NA_RamToStack_3To2Padded)
          )
          ; SIZE already set in csd-eq-one-type5
          ;(eq SIZE (+ 1 NIB_2))
        )
        ;[2] == 1 && [3] == 1
        (eq MICRO_INST NA_RamToStack_3To2Full)
      )
    )
  )
)

(defun (micro-instruction-writing-type5)
  (if-eq IS_MICRO 1
    (begin
      (csd-eq-one-type5)
      (if-zero TOT
        (if-eq-else CSD 1
          (if-zero-else FAST
          (eq MICRO_INST Exceptional_RamToStack_3To2Full)
          (eq MICRO_INST ExceptionalRamToStack3To2FullFast)
          )
        (if-zero-else ALIGNED
          (micro-instruction-writing-not-aligned)
          (micro-instruction-writing-aligned)
        )
        )
      )
    )
  )
)

;====== type5 ======
(defconstraint type5 ()
  (if-eq PRE type5
    (begin
      (preprocessing-type5)
      (micro-instruction-writing-type5)
    )
  )
)

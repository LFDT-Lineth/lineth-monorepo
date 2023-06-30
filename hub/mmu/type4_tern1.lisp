(module mmu)

; === PREPROCESSING ===

(defun (euc-division-tern1)
  (begin
    (eq (+ REFO OFF_2_LO) (+ (* 16 ACC_3) NIB_3))
    (eq (+ REFO (- REFS 1)) (+ (* 16 ACC_4) NIB_4))
    (eq OFF_1_LO (+ (* 16 ACC_5) NIB_5))
    (eq (+ OFF_1_LO  (- REFS OFF_2_LO 1)) (+ (* 16 ACC_6) NIB_6))
    (eq (+ OFF_1_LO (- REFS OFF_2_LO)) (+ (* 16 ACC_7) NIB_7))
    (eq (+ OFF_1_LO (- SIZE_IMPORTED 1)) (+ (* 16 ACC_8) NIB_8))
  )
)

(defun (comparisions-tern1)
  (begin
    (eq NIB_1 (- (* (- NIB_5 NIB_3)
                    (- (* 2 BIT_1) 1)
              ) BIT_1 ))

    (eq NIB_2 (- (* (- NIB_4 NIB_6)
                    (- (* 2 BIT_2) 1)
              ) BIT_2 ))
  )
)

(defun (workflow-tern1)
  (begin
    (eq TOT (+ (- ACC_4 ACC_3) 1 (- ACC_8  ACC_7) 1))
    (eq TOTRD (+ (- ACC_4 ACC_3) 1))
    (eq TOTPD (+ (- ACC_8 ACC_7) 1))

    (if-eq-else NIB_3 NIB_5
      (eq ALIGNED 1)
      (vanishes! ALIGNED)
    )

    (if-eq-else TOTRD 1
      (eq BIT_3 1)
      (vanishes! BIT_3)
    )

    (if-eq-else TOTPD 1
      (eq BIT_4 1)
      (vanishes! BIT_4)
    )

    (if-eq-else NIB_6 15
      (eq BIT_5 1)
      (vanishes! BIT_5)
    )

    (if-eq-else NIB_8 15
      (eq BIT_6 1)
      (vanishes! BIT_6)
    )

    (if-zero-else BIT_3
      (vanishes! BIT_7)
      (eq NIB_9 (+ NIB_5 (- (- NIB_4 NIB_3) (* 16 BIT_7)) ))
    )
  )
)

(defun (offsets-tern1)
  (begin
    (will-eq! SLO SLO)
    (eq SLO ACC_3)

    (will-eq! SBO SBO)
    (eq SBO NIB_3)

    (will-eq! TLO TLO)
    (eq TLO ACC_5)

    (will-eq! TBO TBO)
    (eq TBO NIB_5)
  )
)

;4.8.1
(defun (preprocessing-type4-tern1)
  (if-eq TERNARY tern1
    (begin
        (euc-division-tern1)
        (comparisions-tern1)
        (workflow-tern1)
        (offsets-tern1)
    )
  )
)

; === MICRO-INSTRUCTION-RITING ===

(defun (micro-instruction-writing-type4-tern1)
  (if-eq TERNARY tern1
    (begin
     (micro-instruction-writing-type4-tern1-updating-totrd)
     (micro-instruction-writing-type4-tern1-data-extraction)
     (micro-instruction-writing-type4-tern1-zero-padding)
    )
  )
)


;4.8.2 ======
(defun (micro-instruction-writing-type4-tern1-updating-totrd)
    (if-eq IS_MICRO 1
      (if-zero-else (shift TOTRD -1)
        ;2
        (vanishes! TOTRD)
        ;1
        (eq TOTRD (- (prev TOTRD) 1))

      )
    )
)

;4.8.3 ======

(defun (type4-tern1-data-extraction-no-overlapping)
  (begin
    (if-eq PRE type4CC
      (eq MICRO_INST ExoToRamSlideChunk)
    )
    (if-eq PRE type4RD
      (eq MICRO_INST RamToRamSlideChunk)
    )
    (if-eq PRE type4CD
      (if-zero-else INFO
       (eq MICRO_INST RamToRamSlideChunk)
       (eq MICRO_INST ExoToRamSlideChunk)
      )
    )
  )
)

(defun (type4-tern1-data-extraction-overlapping)
  (begin
    (if-eq PRE type4CC
      (eq MICRO_INST ExoToRamSlideOverlappingChunk)
    )
    (if-eq PRE type4RD
      (eq MICRO_INST RamToRamSlideOverlappingChunk)
    )
    (if-eq PRE type4CD
      (if-zero-else INFO
        (eq MICRO_INST RamToRamSlideOverlappingChunk)
        (eq MICRO_INST ExoToRamSlideOverlappingChunk)
      )
    )
  )
)

;1.
(defun (micro-instruction-writing-type4-tern1-data-extraction-bit3-set)
  (begin
    ;c
    (eq SIZE (+ (- NIB_4 NIB_3) 1))
    ;d
      (begin
        (if-zero-else BIT_7
          (begin
            ;i

            (type4-tern1-data-extraction-no-overlapping)
            (will-inc! TLO BIT_5)
            (will-eq! TBO NIB_7)
          )
          ;ii
          (begin

            (type4-tern1-data-extraction-overlapping)
            (will-inc! TLO 1)
            (will-eq! TBO NIB_7)
          )
        )
    )
  )
)


;2
(defun (micro-instruction-writing-type4-tern1-data-extraction-bit3-not-set)
  (begin
  ;  ;a
    (eq SLO (+ (shift SLO -1) (shift IS_MICRO -1)))
  ;  ;b
    (if-zero (shift IS_MICRO -1)
      (will-eq! TLO (+ TLO ALIGNED BIT_1))
    )
    ;c
    (if-not-zero (shift IS_MICRO -1)
      (if-not-zero TOTRD
        (will-eq! TLO (+ TLO 1))
      ;0
      )
    )

    (if-zero-else (shift IS_MICRO -1)
      ;d
      (begin

       (eq SIZE (+ (- 15 NIB_3) 1))
       (if-zero-else BIT_1
         ;ii
         (type4-tern1-data-extraction-no-overlapping)
         ;iii
         (type4-tern1-data-extraction-overlapping)
       )
      )
      ;e
      (if-not-zero (prev TOTRD)
        (begin
          ;i
         (vanishes! SBO)
          ;ii

         (eq TBO (- (+ (+ NIB_5 (- 15 NIB_3)) 1 )
                     (* 16 (+ ALIGNED BIT_1))
                 )
         )

        (if-zero-else TOTRD
        ;TOTRD_{i} == 0
          (begin
            (eq SIZE (+ NIB_4 1))
            (if-zero-else BIT_2
              (begin
                (type4-tern1-data-extraction-no-overlapping)
                (will-inc! TLO BIT_5)
                (will-eq! TBO NIB_7)
              )
              (begin
                (type4-tern1-data-extraction-overlapping)
                (will-inc! TLO 1)
                (will-eq! TBO NIB_7)
              )
            )
          )
          ;TOTRD_{i} != 0
          (begin
            (eq SIZE 16)
            (if-zero-else ALIGNED
              (begin
                (if-eq PRE type4CC
                  (eq MICRO_INST ExoToRam)
                )
                (if-eq PRE type4RD
                  (eq MICRO_INST RamToRam)
                )
                (if-eq PRE type4CD
                  (if-zero-else INFO
                    (eq MICRO_INST RamToRam)
                    (eq MICRO_INST ExoToRam)
                  )
                )
              )
              (type4-tern1-data-extraction-overlapping)
            )
          )
        )
      )
    )
  )
))

(defun (micro-instruction-writing-type4-tern1-data-extraction)
    (if-eq IS_MICRO 1
      (if-not-zero (shift TOTRD -1)
        (if-zero-else BIT_3
          (micro-instruction-writing-type4-tern1-data-extraction-bit3-not-set)
          (micro-instruction-writing-type4-tern1-data-extraction-bit3-set)
        )
      )
    )
)

;4.8.4 ======
(defun (micro-instruction-writing-type4-tern1-zero-padding)
    (if-eq IS_MICRO 1
      (if-zero (shift TOTRD -1)
        (if-zero-else BIT_4
        ;1.
        (micro-instruction-writing-type4-tern1-zero-padding-bit4-false)
        ;2.
        (micro-instruction-writing-type4-tern1-zero-padding-bit4-true)
      )
    )
  )
)

(defun (micro-instruction-writing-type4-tern1-zero-padding-bit4-false)
  (if-zero-else (shift TOTRD -2)
    ;b
    (begin
      ;i
      (eq TLO (+ (shift TLO -1) 1))
      ;ii
      (vanishes! TBO)

      (if-zero-else TOT
        ;iv
        (if-zero-else BIT_6
          (begin
            (eq SIZE (+ NIB_8 1))
            (eq MICRO_INST RamLimbExcision)
          )
          (eq MICRO_INST KillingOne)

        )

        ;iii
        (eq MICRO_INST KillingOne)
      )


    )

    ;a
    (begin
      (eq TBO NIB_7)
      (eq SIZE (- 16 NIB_7))
      (if-zero-else BIT_5
        (eq MICRO_INST RamLimbExcision)
        (eq MICRO_INST KillingOne)

      )
    )
  )
)

(defun (micro-instruction-writing-type4-tern1-zero-padding-bit4-true)
  ;b
  (begin
    (vanishes! SLO)
    (vanishes! SBO)
    (if-zero-else (* BIT_5 BIT_6)
      ;d
      (begin
        ;i.
        (eq TBO NIB_7)
        ;ii.
        (eq SIZE (+ (- NIB_8 NIB_7) 1))
        ;iii.
        (eq MICRO_INST RamLimbExcision)
      )
      ;c
      (eq MICRO_INST KillingOne)
    )
  )
)

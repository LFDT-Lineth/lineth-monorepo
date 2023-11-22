(module mmu)

;====== precomputation ======

(defun (euclidian-division-type3)
  (begin
    ;OFF_1 = 16 * ACC_1 + NIB_1
    (eq OFF_1_LO
      (+
        (* 16 ACC_1)
        NIB_1
      )
    )

    ;<SIZE> = 16 * ACC_2 + NIB_2
    (eq SIZE_IMPORTED
      (+
        (* 16 ACC_2)
        NIB_2
      )
    )
  )
)

(defun (special-final-type3)
  (if-zero-else NIB_2
    (vanishes! BIT_1)
    (eq BIT_1 1)
  )
)

(defun (fast-operation-type3)
  (begin
    (if-zero-else NIB_1
      (eq ALIGNED 1)
      (vanishes! ALIGNED)
    )
  )
)

(defun (nature-of-final-micro-inst-type3)
  (begin
     (if-zero-else ALIGNED
       ; NIB_3 = NIB_1 + (NIB_2 - 1) - 16 * BIT_2
       (eq NIB_3 (- ( + NIB_1 (- NIB_2 1) ) (* 16 BIT_2)) )
       (vanishes! BIT_2)
     )
  )
)

(defun (workflow-type3)
  (begin
    (eq TOT (+ ACC_2 BIT_1))
    (eq SLO ACC_1)
    (eq SBO NIB_1)
    (vanishes! TLO)
    (vanishes! TBO)
  )
)

(defun (preprocessing-type3)
  (if-zero IS_MICRO
    (if-eq (next IS_MICRO) 1
      (begin
        (euclidian-division-type3)
        (fast-operation-type3)
        (special-final-type3)
        (nature-of-final-micro-inst-type3)
        (workflow-type3)
      )
    )
  )
)

; ====== micro-instruction-writing ======

(defun (set-source-target-limb-and-byte-type3)
  (begin
    (eq SLO (+ (shift SLO -1) (shift IS_MICRO -1)))
    (eq SBO NIB_1)

    (eq TLO (+ (shift TLO -1) (shift IS_MICRO -1)))
    (vanishes! TBO)
  )
)

(defun (tot-type3)
                (if-zero-else TOT
                    (if-eq-else (* ALIGNED (- 1 BIT_1)) 1
                        (begin
                            (eq MICRO_INST RamIsExo)
                            (eq FAST 1))
                        (begin
                            (eq SIZE NIB_2)
                            (if-zero-else BIT_2
                                (begin
                                    (eq MICRO_INST PaddedExoFromOne)
                                    (vanishes! FAST))
                                (begin
                                    (eq MICRO_INST PaddedExoFromTwo)
                                    (vanishes! FAST)))))
                    (if-zero-else ALIGNED
                        (begin
                            (eq MICRO_INST FullExoFromTwo)
                            (vanishes! FAST))
                        (begin
                            (eq MICRO_INST RamIsExo)
                            (eq FAST 1)))))

(defun (micro-instruction-writing-type3)
  (if-eq IS_MICRO 1
  (begin
    (set-source-target-limb-and-byte-type3)
    (tot-type3)
  ))
)

;====== type3 ======

(defconstraint type3 ()
  (if-eq PRE type3
    (begin
      (eq CN_S CN)
      (vanishes! CN_T)
      (vanishes! OFFOOB)
      (preprocessing-type3)
      (micro-instruction-writing-type3)
    )
  )
)

(module mmu)

(defconst;ants
    MSTORE		82
    MSTORE8		83)

(defunalias if-zero-else if-zero)


;====== precomputation ======

(defun (source-and-target-cn-type1)
  (if-zero-else TO_RAM
    ;TO_RAM == 0
    (begin
      (eq CN_S CN)
      (vanishes! CN_T))
    ;TO_RAM == 1
    (begin
      (vanishes! CN_S)
      (eq CN_T CN))
  )
)

(defun (off_1-decomposition-type1)
    (eq OFF_1_LO  (+  (* 16 ACC_1)  NIB_1)))

(defun (limb-and-byte-offsets-type1)
  (if-zero-else TO_RAM
    ;TO_RAM == 0
    (begin
      (eq (shift SLO 1) SLO)
      (eq SLO ACC_1)

      (eq (shift SBO 1) SBO)
      (eq SBO NIB_1)

      (vanishes! (shift TLO 1))
      (vanishes! TLO)
      (vanishes! (shift TBO 1))
      (vanishes! TBO)
    )
    ;TO_RAM == 1
    (begin
      (vanishes! (shift SLO 1))
      (vanishes! SLO)
      (vanishes! (shift SBO 1))
      (vanishes! SBO)

      (eq (shift TLO 1) TLO)
      (eq TLO ACC_1)

      (eq (shift TBO 1) TBO)
      (eq TBO NIB_1)
   )
  )
)

(defun (operation-flag-type1)
  (if-zero-else NIB_1
    (eq ALIGNED 1)
    (vanishes! ALIGNED)
  )
)

(defconstraint preprocessiing ()
  (if-eq PRE type1
    (if-zero IS_MICRO
      (if-eq (shift IS_MICRO 1) 1
        (begin
          ;1
          (vanishes! OFFOOB)
          ;2
          (source-and-target-cn-type1)
          ;3
          (off_1-decomposition-type1)
          ;4
          (limb-and-byte-offsets-type1)
          ;5
          (operation-flag-type1)
          ;6
          (eq TOT 1)
        ))
    )))

;====== micro-instruction-writing ======

(defun (micro-inst-type1)
  (if-zero-else ALIGNED
    ;ALIGNED == 0
    (if-zero-else TO_RAM
      (begin
         (eq MICRO_INST NA_RamToStack_3To2Full)
         (vanishes! FAST)
      )
      (if-eq INST MSTORE
        (begin
          (eq MICRO_INST FullStackToRam)
          (vanishes! FAST)
        )
      )
    )

    ;ALIGNED == 1
    (if-zero-else TO_RAM
      (begin
        (eq MICRO_INST PushTwoRamToStack)
        (eq FAST 1)
      )
      (if-eq INST MSTORE
        (begin
          (eq MICRO_INST PushTwoStackToRam)
          (eq FAST 1)
        )
      )
    )
  )
)

(defconstraint micro-instruction-writing ()
  (if-eq PRE type1
    (if-eq IS_MICRO 1
      (begin
        (micro-inst-type1)
        (if-eq INST MSTORE8
          (begin
            (eq MICRO_INST LsbFromStackToRAM)
            (vanishes! FAST)
          )
        )
      )
    )
  )
)

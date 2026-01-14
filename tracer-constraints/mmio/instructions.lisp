(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;  MMIO instruction constraints  ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint limb-vanishes (:guard IS_LIMB_VANISHES)
               (begin (vanishes! CN_A)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_X TLO)
                      (vanishes! LIMB)))

(defconstraint limb-to-ram-transplant (:guard IS_LIMB_TO_RAM_TRANSPLANT)
               (begin (eq! CN_A CNT)
                      (vanishes! CN_B)
                      (vanishes! CN_B)
                      (eq! INDEX_A TLO)
                      (eq! VAL_A_NEW LIMB)
                      (eq! INDEX_X SLO)))

(defconstraint limb-to-ram-one-target (:guard IS_LIMB_TO_RAM_ONE_TARGET)
               (begin (eq! CN_A CNT)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_A TLO)
                      (eq! INDEX_X SLO)
                      (one-partial-to-one    VAL_A
                                             VAL_A_NEW
                                             BYTE_LIMB
                                             BYTE_A
                                             [ACC 1]
                                             [ACC 2]
                                             [POW_256 1]
                                             SBO
                                             TBO
                                             SIZE
                                             [BIT 1]
                                             [BIT 2]
                                             [BIT 3]
                                             [BIT 4]
                                             CT)))

(defconstraint limb-to-ram-two-target (:guard IS_LIMB_TO_RAM_TWO_TARGET)
               (begin (eq! CN_A CNT)
                      (eq! CN_B CNT)
                      (vanishes! CN_C)
                      (eq! INDEX_A TLO)
                      (eq! INDEX_B (+ TLO 1))
                      (eq! INDEX_X SLO)
                      (one-partial-to-two VAL_A
                                          VAL_B
                                          VAL_A_NEW
                                          VAL_B_NEW
                                          BYTE_LIMB
                                          BYTE_A
                                          BYTE_B
                                          [ACC 1]
                                          [ACC 2]
                                          [ACC 3]
                                          [ACC 4]
                                          [POW_256 1]
                                          SBO
                                          TBO
                                          SIZE
                                          [BIT 1]
                                          [BIT 2]
                                          [BIT 3]
                                          [BIT 4]
                                          [BIT 5]
                                          CT)))

(defconstraint ram-to-limb-transplant (:guard IS_RAM_TO_LIMB_TRANSPLANT)
               (begin (eq! CN_A CNS)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_A SLO)
                      (eq! VAL_A_NEW VAL_A)
                      (eq! INDEX_X TLO)
                      (eq! LIMB VAL_A)))

(defconstraint ram-to-limb-one-source (:guard IS_RAM_TO_LIMB_ONE_SOURCE)
               (begin (eq! CN_A CNS)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_A SLO)
                      (eq! VAL_A_NEW VAL_A)
                      (eq! INDEX_X TLO)
                      (one-to-one-padded    LIMB
                                            BYTE_A
                                            [ACC 1]
                                            [POW_256 1]
                                            SBO
                                            TBO
                                            SIZE
                                            [BIT 1]
                                            [BIT 2]
                                            [BIT 3]
                                            CT)))


(defconstraint ram-to-limb-two-source (:guard IS_RAM_TO_LIMB_TWO_SOURCE)
               (begin (eq! CN_A CNS)
                      (eq! CN_B CNS)
                      (vanishes! CN_C)
                      (eq! INDEX_A SLO)
                      (eq! INDEX_B (+ SLO 1))
                      (eq! VAL_A_NEW VAL_A)
                      (eq! VAL_B_NEW VAL_B)
                      (eq! INDEX_X TLO)
                      (two-to-one-padded LIMB
                                         BYTE_A
                                         BYTE_B
                                         [ACC 1]
                                         [ACC 2]
                                         [POW_256 1]
                                         [POW_256 2]
                                         SBO
                                         TBO
                                         SIZE
                                         [BIT 1]
                                         [BIT 2]
                                         [BIT 3]
                                         [BIT 4]
                                         CT)))

(defconstraint ram-to-ram-transplant (:guard IS_RAM_TO_RAM_TRANSPLANT)
               (begin (eq! CN_A CNS)
                      (eq! CN_B CNT)
                      (vanishes! CN_C)
                      (eq! INDEX_A SLO)
                      (eq! INDEX_B TLO)
                      (eq! VAL_A_NEW VAL_A)
                      (eq! VAL_B_NEW VAL_A)))

(defconstraint ram-to-ram-partial (:guard IS_RAM_TO_RAM_PARTIAL)
               (begin (eq! CN_A CNS)
                      (eq! CN_B CNT)
                      (vanishes! CN_C)
                      (eq! INDEX_A SLO)
                      (eq! INDEX_B TLO)
                      (eq! VAL_A_NEW VAL_A)
                      (one-partial-to-one    VAL_B
                                             VAL_B_NEW
                                             BYTE_A
                                             BYTE_B
                                             [ACC 1]
                                             [ACC 2]
                                             [POW_256 1]
                                             SBO
                                             TBO
                                             SIZE
                                             [BIT 1]
                                             [BIT 2]
                                             [BIT 3]
                                             [BIT 4]
                                             CT)))

(defconstraint ram-to-ram-two-target (:guard IS_RAM_TO_RAM_TWO_TARGET)
               (begin (eq! CN_A CNS)
                      (eq! CN_B CNT)
                      (eq! CN_C CNT)
                      (eq! INDEX_A SLO)
                      (eq! INDEX_B TLO)
                      (eq! INDEX_C (+ TLO 1))
                      (eq! VAL_A_NEW VAL_A)
                      (one-partial-to-two    VAL_B
                                             VAL_C
                                             VAL_B_NEW
                                             VAL_C_NEW
                                             BYTE_A
                                             BYTE_B
                                             BYTE_C
                                             [ACC 1]
                                             [ACC 2]
                                             [ACC 3]
                                             [ACC 4]
                                             [POW_256 1]
                                             SBO
                                             TBO
                                             SIZE
                                             [BIT 1]
                                             [BIT 2]
                                             [BIT 3]
                                             [BIT 4]
                                             [BIT 5]
                                             CT)))

;; This is just a way to cast an intermediate result, as the current constraint were creating a i354 which makes the splitting huge.

(defcomputedcolumn (CAST_INTERMEDIATE_RESULT :i128 :fwd) (* IS_RAM_TO_RAM_TWO_SOURCE [ACC 1] [POW_256 2]))

(defconstraint ram-to-ram-two-source (:guard IS_RAM_TO_RAM_TWO_SOURCE)
               (begin (eq! CN_A CNS)
                      (eq! CN_B CNS)
                      (eq! CN_C CNT)
                      (eq! INDEX_A SLO)
                      (eq! INDEX_B (+ SLO 1))
                      (eq! INDEX_C TLO)
                      (eq! VAL_A_NEW VAL_A)
                      (eq! VAL_B_NEW VAL_B)
                      (two-partial-to-one    VAL_C
                                             VAL_C_NEW
                                             BYTE_A
                                             BYTE_B
                                             BYTE_C
                                             [ACC 1]
                                             [ACC 2]
                                             [ACC 3]
                                             [POW_256 1]
                                             [POW_256 2]
                                             SBO
                                             TBO
                                             SIZE
                                             [BIT 1]
                                             [BIT 2]
                                             [BIT 3]
                                             [BIT 4]
                                             CT
                                             CAST_INTERMEDIATE_RESULT)))

;; original constraint:                                                      
;; 
;; (defconstraint ram-to-ram-two-source (:guard IS_RAM_TO_RAM_TWO_SOURCE)
;;                (begin (eq! CN_A CNS)
;;                       (eq! CN_B CNS)
;;                       (eq! CN_C CNT)
;;                       (eq! INDEX_A SLO)
;;                       (eq! INDEX_B (+ SLO 1))
;;                       (eq! INDEX_C TLO)
;;                       (eq! VAL_A_NEW VAL_A)
;;                       (eq! VAL_B_NEW VAL_B)
;;                       (two-partial-to-one    VAL_C
;;                                              VAL_C_NEW
;;                                              BYTE_A
;;                                              BYTE_B
;;                                              BYTE_C
;;                                              [ACC 1]
;;                                              [ACC 2]
;;                                              [ACC 3]
;;                                              [POW_256 1]
;;                                              [POW_256 2]
;;                                              SBO
;;                                              TBO
;;                                              SIZE
;;                                              [BIT 1]
;;                                              [BIT 2]
;;                                              [BIT 3]
;;                                              [BIT 4]
;;                                              CT)))

(defconstraint ram-excision (:guard IS_RAM_EXCISION)
               (begin (eq! CN_A CNT)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_A TLO)
                      (excision    VAL_A
                                   VAL_A_NEW
                                   BYTE_A
                                   [ACC 1]
                                   [POW_256 1]
                                   TBO
                                   SIZE
                                   [BIT 1]
                                   [BIT 2]
                                   CT)))

(defconstraint ram-vanishes (:guard IS_RAM_VANISHES)
               (begin (eq! CN_A CNT)
                      (vanishes! CN_B)
                      (vanishes! CN_C)
                      (eq! INDEX_A TLO)
                      (vanishes! VAL_A_NEW)))

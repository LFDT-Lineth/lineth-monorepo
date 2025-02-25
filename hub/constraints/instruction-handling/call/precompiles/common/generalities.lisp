(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.Y.Z.1 Introduction      ;;
;;    X.Y.Z.2 Representation    ;;
;;    X.Y.Z.3 Generalities      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---common---precondition)    (*    PEEK_AT_SCENARIO    (scenario-shorthand---PRC---common-address-bit-sum)))


(defconstraint    precompile-processing---common---setting-MISC-module-flags    (:guard     (precompile-processing---common---precondition))
                  (eq!    (weighted-MISC-flag-sum    1)
                          (+    (*    MISC_WEIGHT_MMU    (precompile-processing---common---OOB-extract-call-data))
                                MISC_WEIGHT_OOB)))

(defconstraint    precompile-processing---common---setting-OOB-instruction    (:guard    (precompile-processing---common---precondition))
                  (set-OOB-instruction---common    precompile-processing---common---1st-misc-row---row-offset  ;; offset
                                                   (precompile-processing---common---OOB-instruction)          ;; relevant OOB instruction
                                                   (precompile-processing---dup-call-gas)                      ;; call gas i.e. gas provided to the precompile
                                                   (precompile-processing---dup-cds)                           ;; call data size
                                                   (precompile-processing---dup-r@c)                           ;; return at capacity
                                                   )
                  )

(defun    (precompile-processing---common---OOB-instruction)
  (+    (*    OOB_INST_ECRECOVER    scenario/PRC_ECRECOVER    )
        (*    OOB_INST_SHA2         scenario/PRC_SHA2-256     )
        (*    OOB_INST_RIPEMD       scenario/PRC_RIPEMD-160   )
        (*    OOB_INST_IDENTITY     scenario/PRC_IDENTITY     )
        (*    OOB_INST_ECADD        scenario/PRC_ECADD        )
        (*    OOB_INST_ECMUL        scenario/PRC_ECMUL        )
        (*    OOB_INST_ECPAIRING    scenario/PRC_ECPAIRING    )
        ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; OOB related shorthands ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (precompile-processing---common---OOB-hub-success)          (shift    [misc/OOB_DATA    4]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-return-gas)           (shift    [misc/OOB_DATA    5]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-extract-call-data)    (shift    [misc/OOB_DATA    6]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-empty-call-data)      (shift    [misc/OOB_DATA    7]    precompile-processing---common---1st-misc-row---row-offset))
(defun    (precompile-processing---common---OOB-r@c-nonzero)          (shift    [misc/OOB_DATA    8]    precompile-processing---common---1st-misc-row---row-offset)) ;; ""

(defconstraint    precompile-processing---common---setting-MMU-instruction    (:guard    (precompile-processing---common---precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---common---1st-misc-row---row-offset)
                                  (begin
                                    (if-not-zero    scenario/PRC_IDENTITY
                                                    (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---common---1st-misc-row---row-offset   ;; offset
                                                                                                      CONTEXT_NUMBER                                               ;; source ID
                                                                                                      (+    1    HUB_STAMP)                                        ;; target ID
                                                                                                      ;; aux_id                                                    ;; auxiliary ID
                                                                                                      ;; src_offset_hi                                             ;; source offset high
                                                                                                      (precompile-processing---dup-cdo)                            ;; source offset low
                                                                                                      ;; tgt_offset_lo                                             ;; target offset low
                                                                                                      (precompile-processing---dup-cds)                            ;; size
                                                                                                      0                                                            ;; reference offset
                                                                                                      (precompile-processing---dup-cds)                            ;; reference size
                                                                                                      ;; success_bit                                               ;; success bit
                                                                                                      ;; limb_1                                                    ;; limb 1
                                                                                                      ;; limb_2                                                    ;; limb 2
                                                                                                      ;; exo_sum                                                   ;; weighted exogenous module flag sum
                                                                                                      ;; phase                                                     ;; phase
                                                                                                      ))
                                    (if-not-zero    (scenario-shorthand---PRC---common-except-identity-address-bit-sum)
                                                    (set-MMU-instruction---ram-to-exo-with-padding    precompile-processing---common---1st-misc-row---row-offset   ;; offset
                                                                                                      CONTEXT_NUMBER                                               ;; source ID
                                                                                                      (+    1    HUB_STAMP)                                        ;; target ID
                                                                                                      0                                                            ;; auxiliary ID (here: ∅)
                                                                                                      ;; src_offset_hi                                                ;; source offset high
                                                                                                      (precompile-processing---dup-cdo)                            ;; source offset low
                                                                                                      ;; tgt_offset_lo                                                ;; target offset low
                                                                                                      (precompile-processing---dup-cds)                            ;; size
                                                                                                      ;; ref_offset                                                   ;; reference offset
                                                                                                      (precompile-processing---common---MMU-reference-size)        ;; reference size
                                                                                                      (precompile-processing---common---MMU-success-bit)           ;; success bit (TODO: ugly self referential constraint ...)
                                                                                                      ;; limb_1                                                       ;; limb 1
                                                                                                      ;; limb_2                                                       ;; limb 2
                                                                                                      (precompile-processing---common---MMU-exo-sum)               ;; weighted exogenous module flag sum
                                                                                                      (precompile-processing---common---MMU-phase)                 ;; phase
                                                                                                      ))
                                  )
                  ))

(defun    (precompile-processing---common---MMU-success-bit)
  (shift    misc/MMU_SUCCESS_BIT    precompile-processing---common---1st-misc-row---row-offset))

(defun    (precompile-processing---common---MMU-reference-size)
  (+    (*    128                                                    scenario/PRC_ECRECOVER    )
        (*    (precompile-processing---dup-cds)                      scenario/PRC_SHA2-256     )
        (*    (precompile-processing---dup-cds)                      scenario/PRC_RIPEMD-160   )
        (*    128                                                    scenario/PRC_ECADD        )
        (*    96                                                     scenario/PRC_ECMUL        )
        (*    (precompile-processing---dup-cds)                      scenario/PRC_ECPAIRING    )
        ))

(defun    (precompile-processing---common---MMU-exo-sum)
  (+    (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECRECOVER    )
        (*    EXO_SUM_WEIGHT_RIPSHA    scenario/PRC_SHA2-256     )
        (*    EXO_SUM_WEIGHT_RIPSHA    scenario/PRC_RIPEMD-160   )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECADD        )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECMUL        )
        (*    EXO_SUM_WEIGHT_ECDATA    scenario/PRC_ECPAIRING    )
        ))

(defun    (precompile-processing---common---MMU-phase)
  (+    (*    PHASE_ECRECOVER_DATA    scenario/PRC_ECRECOVER    )
        (*    PHASE_SHA2_DATA         scenario/PRC_SHA2-256     )
        (*    PHASE_RIPEMD_DATA       scenario/PRC_RIPEMD-160   )
        (*    PHASE_ECADD_DATA        scenario/PRC_ECADD        )
        (*    PHASE_ECMUL_DATA        scenario/PRC_ECMUL        )
        (*    PHASE_ECPAIRING_DATA    scenario/PRC_ECPAIRING    )
        ))

;; ECRECOVER related shorthands
(defun    (precompile-processing---common---address-recovery-failure)
  (+      (precompile-processing---common---OOB-empty-call-data)
          (*    (precompile-processing---common---OOB-extract-call-data)
                (-    1    (precompile-processing---common---MMU-success-bit)))))
(defun    (precompile-processing---common---address-recovery-success)
  (*      (precompile-processing---common---OOB-extract-call-data)
          (precompile-processing---common---MMU-success-bit)))

;; ECADD, ECMUL and ECPAIRING related shorthands
(defun    (precompile-processing---common---malformed-data)     (*    (precompile-processing---common---OOB-extract-call-data)
                                                                      (-   1    (precompile-processing---common---MMU-success-bit))))
(defun    (precompile-processing---common---wellformed-data)    (+    (precompile-processing---common---OOB-empty-call-data)
                                                                      (*    (precompile-processing---common---OOB-extract-call-data)
                                                                            (precompile-processing---common---MMU-success-bit))))


;; (defconstraint    precompile-processing---common---debug-constraints-for-address-recovery                   (:guard    (precompile-processing---common---precondition)))
;; (defconstraint    precompile-processing---common---debug-constraints-for-data-integrity                     (:guard    (precompile-processing---common---precondition)))
;; (defconstraint    precompile-processing---common---debug-constraints-for-automatic-success-bit-vanishing    (:guard    (precompile-processing---common---precondition)))

(defconstraint    precompile-processing---common---justifying-success-scenario    (:guard    (precompile-processing---common---precondition))
                  (eq!    (scenario-shorthand---PRC---success)
                          (+    (*    (precompile-processing---common---OOB-hub-success)
                                      (+    scenario/PRC_ECRECOVER
                                            scenario/PRC_SHA2-256
                                            scenario/PRC_RIPEMD-160
                                            scenario/PRC_IDENTITY
                                            ))
                                (*    (precompile-processing---common---wellformed-data)
                                      (+    scenario/PRC_ECADD
                                            scenario/PRC_ECMUL
                                            scenario/PRC_ECPAIRING
                                            ))
                                )))

(defconstraint    precompile-processing---common---justifying-FAILURE_KNOWN_TO_HUB    (:guard    (precompile-processing---common---precondition))
                  (eq!    scenario/PRC_FAILURE_KNOWN_TO_HUB
                          (*    (-    1    (precompile-processing---common---OOB-hub-success))
                                (scenario-shorthand---PRC---common-address-bit-sum)
                                )))

(defconstraint    precompile-processing---common---justifying-FAILURE_KNOWN_TO_RAM    (:guard    (precompile-processing---common---precondition))
                  (eq!    scenario/PRC_FAILURE_KNOWN_TO_RAM
                          (*    (precompile-processing---common---malformed-data)
                                (+    scenario/PRC_ECADD
                                      scenario/PRC_ECMUL
                                      scenario/PRC_ECPAIRING))))

(defconstraint    precompile-processing---common---justifying-return-gas-prediction    (:guard    (precompile-processing---common---precondition))
                  (begin
                    (if-not-zero    (scenario-shorthand---PRC---failure)
                                    (vanishes!    (precompile-processing---prd-return-gas)))
                    (if-not-zero    (scenario-shorthand---PRC---success)
                                    (eq!          (precompile-processing---prd-return-gas)
                                                  (precompile-processing---common---OOB-return-gas)))
                    ))

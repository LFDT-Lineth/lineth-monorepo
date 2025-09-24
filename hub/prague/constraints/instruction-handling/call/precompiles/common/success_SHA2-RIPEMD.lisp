(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                                    ;;
;;    X.Y.Z.5 The  PRC/SHA2-256 and  PRC/RIPEMD-160 success case    ;;
;;                                                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---SHA2-and-RIPEMD---success-precondition)    (*    PEEK_AT_SCENARIO
                                                                                 (+    scenario/PRC_SHA2-256
                                                                                       scenario/PRC_RIPEMD-160)
                                                                                 (scenario-shorthand---PRC---success)))

(defconstraint    precompile-processing---SHA2-and-RIPEMD---success---2nd-misc-row---setting-module-flags       (:guard    (precompile-processing---SHA2-and-RIPEMD---success-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---common---2nd-misc-row---row-offset)
                          MISC_WEIGHT_MMU))

(defconstraint    precompile-processing---SHA2-and-RIPEMD---success---2nd-misc-row---setting-MMU-instruction    (:guard    (precompile-processing---SHA2-and-RIPEMD---success-precondition))
                  (begin
                    (if-not-zero    (precompile-processing---common---OOB-empty-call-data)
                                    (set-MMU-instruction---mstore    precompile-processing---common---2nd-misc-row---row-offset               ;; offset
                                                                     ;; src_id                                                                   ;; source ID
                                                                     (+    1    HUB_STAMP)                                                    ;; target ID
                                                                     ;; aux_id                                                                   ;; auxiliary ID
                                                                     ;; src_offset_hi                                                            ;; source offset high
                                                                     ;; src_offset_lo                                                            ;; source offset low
                                                                     0                                                                        ;; target offset low
                                                                     ;; size                                                                     ;; size
                                                                     ;; ref_offset                                                               ;; reference offset
                                                                     ;; ref_size                                                                 ;; reference size
                                                                     ;; success_bit                                                              ;; success bit
                                                                     (precompile-processing---SHA2-and-RIPEMD---relevant-empty-hash-hi)       ;; limb 1
                                                                     (precompile-processing---SHA2-and-RIPEMD---relevant-empty-hash-lo)       ;; limb 2
                                                                     ;; exo_sum                                                                  ;; weighted exogenous module flag sum
                                                                     ;; phase                                                                    ;; phase
                                                                     ))
                    (if-not-zero    (precompile-processing---common---OOB-extract-call-data)
                                    (set-MMU-instruction---exo-to-ram-transplants    precompile-processing---common---2nd-misc-row---row-offset               ;; offset
                                                                                     (+    1    HUB_STAMP)                                                    ;; source ID
                                                                                     (+    1    HUB_STAMP)                                                    ;; target ID
                                                                                     ;; aux_id                                                                   ;; auxiliary ID
                                                                                     ;; src_offset_hi                                                            ;; source offset high
                                                                                     ;; src_offset_lo                                                            ;; source offset low
                                                                                     ;; tgt_offset_lo                                                            ;; target offset low
                                                                                     32                                                                       ;; size
                                                                                     ;; ref_offset                                                               ;; reference offset
                                                                                     ;; ref_size                                                                 ;; reference size
                                                                                     ;; success_bit                                                              ;; success bit
                                                                                     ;; limb_1                                                                   ;; limb 1
                                                                                     ;; limb_2                                                                   ;; limb 2
                                                                                     EXO_SUM_WEIGHT_RIPSHA                                                    ;; weighted exogenous module flag sum
                                                                                     (precompile-processing---SHA2-and-RIPEMD---result-phase)                 ;; phase
                                                                                     ))
                    ))


(defun    (precompile-processing---SHA2-and-RIPEMD---relevant-empty-hash-hi)    (+    (*    EMPTY_SHA2_HI          scenario/PRC_SHA2-256)
                                                                                      (*    EMPTY_RIPEMD_HI        scenario/PRC_RIPEMD-160)))

(defun    (precompile-processing---SHA2-and-RIPEMD---relevant-empty-hash-lo)    (+    (*    EMPTY_SHA2_LO          scenario/PRC_SHA2-256)
                                                                                      (*    EMPTY_RIPEMD_LO        scenario/PRC_RIPEMD-160)))

(defun    (precompile-processing---SHA2-and-RIPEMD---result-phase)              (+    (*    PHASE_SHA2_RESULT      scenario/PRC_SHA2-256)
                                                                                      (*    PHASE_RIPEMD_RESULT    scenario/PRC_RIPEMD-160)))

(defconstraint    precompile-processing---SHA2-and-RIPEMD---success---3rd-misc-row---setting-module-flags       (:guard    (precompile-processing---SHA2-and-RIPEMD---success-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---common---3rd-misc-row---row-offset)
                          (*    MISC_WEIGHT_MMU
                                (precompile-processing---common---OOB-r@c-nonzero))))

(defconstraint    precompile-processing---SHA2-and-RIPEMD---success---3rd-misc-row---setting-MMU-instruction    (:guard    (precompile-processing---SHA2-and-RIPEMD---success-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---common---3rd-misc-row---row-offset)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---common---3rd-misc-row---row-offset                 ;; offset
                                                                                    (+    1    HUB_STAMP)                                                      ;; source ID
                                                                                    CONTEXT_NUMBER                                                             ;; target ID
                                                                                    ;; aux_id                                                                     ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                              ;; source offset high
                                                                                    0                                                                          ;; source offset low
                                                                                    ;; tgt_offset_lo                                                              ;; target offset low
                                                                                    32                                                                         ;; size
                                                                                    (precompile-processing---dup-r@o)                                          ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                          ;; reference size
                                                                                    ;; success_bit                                                                ;; success bit
                                                                                    ;; limb_1                                                                     ;; limb 1
                                                                                    ;; limb_2                                                                     ;; limb 2
                                                                                    ;; exo_sum                                                                    ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                      ;; phase
                                                                                    )
                                  ))

(defconstraint    precompile-processing---SHA2-and-RIPEMD---success---updating-return-data                (:guard    (precompile-processing---SHA2-and-RIPEMD---success-precondition))
                  (provide-return-data     precompile-processing---common---context-row-success---row-offset          ;; row offset
                                           CONTEXT_NUMBER                                                             ;; receiver context
                                           (+    1    HUB_STAMP)                                                      ;; provider context
                                           0                                                                          ;; rdo
                                           32                                                                         ;; rds
                                           ))

(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                      ;;
;;    X.Y.Z.5 ECADD, ECMUL and ECPAIRING constraints    ;;
;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;
;; Shorthands ;;
;;;;;;;;;;;;;;;;

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case)    (*    PEEK_AT_SCENARIO
                                                                                          (+    scenario/PRC_ECADD
                                                                                                scenario/PRC_ECMUL
                                                                                                scenario/PRC_ECPAIRING
                                                                                                (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                                                                                )
                                                                                          (scenario-shorthand---PRC---success)
                                                                                          ))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECADD)          (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECADD))
(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECMUL)          (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECMUL))
(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECPAIRING)      (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECPAIRING))
(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---trivial-ECPAIRING)         (*    (precompile-processing---common---OOB-empty-call-data)      scenario/PRC_ECPAIRING))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-cases)        (+    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECADD)
                                                                                                  (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECMUL)
                                                                                                  (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECPAIRING)
                                                                                                  (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                                                                                  ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 2 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---second-misc-row-peeking-flags
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---full-return-data-transfer)
                          (*    MISC_WEIGHT_MMU    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---trigger_MMU))))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---trigger_MMU)    (+    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECADD)
                                                                                         (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECMUL)
                                                                                         (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECPAIRING)
                                                                                         (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---trivial-ECPAIRING)
                                                                                         (scenario-shorthand---PRC---common-BLS-address-bit-sum)
                                                                                         ))

(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---setting-MMU-instruction---full-return-data-transfer---trivial-case
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (if-not-zero    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---trivial-ECPAIRING)
                                  (set-MMU-instruction---mstore                    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---full-return-data-transfer   ;; offset
                                                                                   ;; src_id                                                                                   ;; source ID
                                                                                   (+    1    HUB_STAMP)                                                                    ;; target ID
                                                                                   ;; aux_id                                                                                   ;; auxiliary ID
                                                                                   ;; src_offset_hi                                                                            ;; source offset high
                                                                                   ;; src_offset_lo                                                                            ;; source offset low
                                                                                   0                                                                                        ;; target offset low
                                                                                   ;; size                                                                                     ;; size
                                                                                   ;; ref_offset                                                                               ;; reference offset
                                                                                   ;; ref_size                                                                                 ;; reference size
                                                                                   ;; success_bit                                                                              ;; success bit
                                                                                   0                                                                                        ;; limb 1
                                                                                   1                                                                                        ;; limb 2
                                                                                   ;; exo_sum                                                                                  ;; weighted exogenous module flag sum
                                                                                   ;; phase                                                                                    ;; phase
                                                                                   )))

(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---setting-MMU-instruction---full-return-data-transfer---nontrivial-cases
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (if-not-zero    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-cases)
                                  (set-MMU-instruction---exo-to-ram-transplants    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---full-return-data-transfer    ;; offset
                                                                                   (+    1    HUB_STAMP)                                                                     ;; source ID
                                                                                   (+    1    HUB_STAMP)                                                                     ;; target ID
                                                                                   ;; aux_id                                                                                    ;; auxiliary ID
                                                                                   ;; src_offset_hi                                                                             ;; source offset high
                                                                                   ;; src_offset_lo                                                                             ;; source offset low
                                                                                   ;; tgt_offset_lo                                                                             ;; target offset low
                                                                                   (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-size)                            ;; size
                                                                                   ;; ref_offset                                                                                ;; reference offset
                                                                                   ;; ref_size                                                                                  ;; reference size
                                                                                   ;; success_bit                                                                               ;; success bit
                                                                                   ;; limb_1                                                                                    ;; limb 1
                                                                                   ;; limb_2                                                                                    ;; limb 2
                                                                                   (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---exo-sum)                             ;; weighted exogenous module flag sum
                                                                                   (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-phase)                           ;; phase
                                                                                   )))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-size)     (+    (*    PRECOMPILE_RETURN_DATA_SIZE___ECADD                  (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECADD)     )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___ECMUL                  (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECMUL)     )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___ECPAIRING              (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECPAIRING) )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___POINT_EVALUATION       scenario/PRC_POINT_EVALUATION                                                          )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_ADD             scenario/PRC_BLS_G1_ADD                                                                )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_MSM             scenario/PRC_BLS_G1_MSM                                                                )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_ADD             scenario/PRC_BLS_G2_ADD                                                                )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_MSM             scenario/PRC_BLS_G2_MSM                                                                )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_PAIRING_CHECK      scenario/PRC_BLS_PAIRING_CHECK                                                         )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP_TO_G1       scenario/PRC_BLS_MAP_FP_TO_G1                                                          )
                                                                                               (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP2_TO_G2      scenario/PRC_BLS_MAP_FP2_TO_G2                                                         )
                                                                                               ))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-phase)              (+    (*    PHASE_ECADD_RESULT                (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECADD)     )
                                                                                                         (*    PHASE_ECMUL_RESULT                (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECMUL)     )
                                                                                                         (*    PHASE_ECPAIRING_RESULT            (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---nontrivial-ECPAIRING) )
                                                                                                         (*    PHASE_POINT_EVALUATION_RESULT     scenario/PRC_POINT_EVALUATION                                                          )
                                                                                                         (*    PHASE_BLS_G1_ADD_RESULT           scenario/PRC_BLS_G1_ADD                                                                )
                                                                                                         (*    PHASE_BLS_G1_MSM_RESULT           scenario/PRC_BLS_G1_MSM                                                                )
                                                                                                         (*    PHASE_BLS_G2_ADD_RESULT           scenario/PRC_BLS_G2_ADD                                                                )
                                                                                                         (*    PHASE_BLS_G2_MSM_RESULT           scenario/PRC_BLS_G2_MSM                                                                )
                                                                                                         (*    PHASE_BLS_PAIRING_CHECK_RESULT    scenario/PRC_BLS_PAIRING_CHECK                                                         )
                                                                                                         (*    PHASE_BLS_MAP_FP_TO_G1_RESULT     scenario/PRC_BLS_MAP_FP_TO_G1                                                          )
                                                                                                         (*    PHASE_BLS_MAP_FP2_TO_G2_RESULT    scenario/PRC_BLS_MAP_FP2_TO_G2                                                         )
                                                                                                         ))


(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---exo-sum)    (+    (*    EXO_SUM_WEIGHT_ECDATA      scenario/PRC_ECADD                                     )
                                                                                     (*    EXO_SUM_WEIGHT_ECDATA      scenario/PRC_ECMUL                                     )
                                                                                     (*    EXO_SUM_WEIGHT_ECDATA      scenario/PRC_ECPAIRING                                 )
                                                                                     (*    EXO_SUM_WEIGHT_BLSDATA     (scenario-shorthand---PRC---common-BLS-address-bit-sum))
                                                                                     ))





;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 3 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---third-misc-row-peeking-flags
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---partial-return-data-copy)
                          (*    MISC_WEIGHT_MMU
                                (precompile-processing---common---OOB-r@c-nonzero))))


(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---setting-the-MMU-instruction---partial-return-data-copy
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (if-not-zero    (shift    misc/MMU_FLAG                           precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---partial-return-data-copy)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---misc-row-offset---partial-return-data-copy  ;; offset
                                                                                    (+    1    HUB_STAMP)                                                                   ;; source ID
                                                                                    CONTEXT_NUMBER                                                                          ;; target ID
                                                                                    ;; aux_id                                                                               ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                                        ;; source offset high
                                                                                    0                                                                                       ;; source offset low
                                                                                    ;; tgt_offset_lo                                                                        ;; target offset low
                                                                                    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-reference-size)                ;; size
                                                                                    (precompile-processing---dup-r@o)                                                       ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                                       ;; reference size
                                                                                    ;; success_bit                                                                          ;; success bit
                                                                                    ;; limb_1                                                                               ;; limb 1
                                                                                    ;; limb_2                                                                               ;; limb 2
                                                                                    ;; exo_sum                                                                              ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                                ;; phase
                                                                                    )))

(defun    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-reference-size)     (+    (*    PRECOMPILE_RETURN_DATA_SIZE___ECADD                 scenario/PRC_ECADD              )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___ECMUL                 scenario/PRC_ECMUL              )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___ECPAIRING             scenario/PRC_ECPAIRING          )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___POINT_EVALUATION      scenario/PRC_POINT_EVALUATION   )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_ADD            scenario/PRC_BLS_G1_ADD         )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_MSM            scenario/PRC_BLS_G1_MSM         )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_ADD            scenario/PRC_BLS_G2_ADD         )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_MSM            scenario/PRC_BLS_G2_MSM         )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_PAIRING_CHECK     scenario/PRC_BLS_PAIRING_CHECK  )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP_TO_G1      scenario/PRC_BLS_MAP_FP_TO_G1   )
                                                                                                         (*    PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP2_TO_G2     scenario/PRC_BLS_MAP_FP2_TO_G2  )
                                                                                                         ))


;;;;;;;;;;;;;;;;;;;;;;;
;; Context-row i + 4 ;;
;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---updating-caller-context-with-precompile-return-data
                  (:guard    (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---success-case))
                  (provide-return-data     precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---context-row-offset---updating-caller-context   ;; row offset
                                           CONTEXT_NUMBER                                                                                         ;; receiver context
                                           (+    1    HUB_STAMP)                                                                                  ;; provider context
                                           0                                                                                                      ;; rdo
                                           (precompile-processing---ECADD-ECMUL-ECPAIRING-and-BLS---return-data-reference-size)                   ;; rds
                                           ))

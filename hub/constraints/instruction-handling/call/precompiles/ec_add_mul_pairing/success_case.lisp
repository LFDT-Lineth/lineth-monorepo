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

(defun    (precompile-processing---ECADD_MUL_PAIRING---success-case)            (*    PEEK_AT_SCENARIO
                                                                                      (+    scenario/PRC_ECADD
                                                                                            scenario/PRC_ECMUL
                                                                                            scenario/PRC_ECPAIRING)
                                                                                      (scenario-shorthand---PRC---success)))

(defun    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD)        (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECADD))
(defun    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL)        (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECMUL))
(defun    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING)    (*    (precompile-processing---common---OOB-extract-call-data)    scenario/PRC_ECPAIRING))
(defun    (precompile-processing---ECADD_MUL_PAIRING---trivial-ECPAIRING)       (*    (precompile-processing---common---OOB-empty-call-data)      scenario/PRC_ECPAIRING))

(defun    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-cases)        (+    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD)
                                                                                      (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL)
                                                                                      (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING)))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 2 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD_MUL_PAIRING---second-misc-row-peeking-flags
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---full-return-data-transfer)
                          (*    MISC_WEIGHT_MMU    (precompile-processing---ECADD_MUL_PAIRING---trigger_MMU))))

(defun    (precompile-processing---ECADD_MUL_PAIRING---trigger_MMU)    (+    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD)
                                                                             (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL)
                                                                             (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING)
                                                                             (precompile-processing---ECADD_MUL_PAIRING---trivial-ECPAIRING)))

(defconstraint    precompile-processing---ECADD_MUL_PAIRING---setting-MMU-instruction---full-return-data-transfer---trivial-case
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (if-not-zero    (precompile-processing---ECADD_MUL_PAIRING---trivial-ECPAIRING)
                                  (set-MMU-instruction---mstore                    precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---full-return-data-transfer   ;; offset
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

(defconstraint    precompile-processing---ECADD_MUL_PAIRING---setting-MMU-instruction---full-return-data-transfer---nontrivial-cases
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (if-not-zero    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-cases)
                                  (set-MMU-instruction---exo-to-ram-transplants    precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---full-return-data-transfer    ;; offset
                                                                                   (+    1    HUB_STAMP)                                                                     ;; source ID
                                                                                   (+    1    HUB_STAMP)                                                                     ;; target ID
                                                                                   ;; aux_id                                                                                    ;; auxiliary ID
                                                                                   ;; src_offset_hi                                                                             ;; source offset high
                                                                                   ;; src_offset_lo                                                                             ;; source offset low
                                                                                   ;; tgt_offset_lo                                                                             ;; target offset low
                                                                                   (precompile-processing---ECADD_MUL_PAIRING---return-data-size)                            ;; size
                                                                                   ;; ref_offset                                                                                ;; reference offset
                                                                                   ;; ref_size                                                                                  ;; reference size
                                                                                   ;; success_bit                                                                               ;; success bit
                                                                                   ;; limb_1                                                                                    ;; limb 1
                                                                                   ;; limb_2                                                                                    ;; limb 2
                                                                                   EXO_SUM_WEIGHT_ECDATA                                                                     ;; weighted exogenous module flag sum
                                                                                   (precompile-processing---ECADD_MUL_PAIRING---return-data-phase)                           ;; phase
                                                                                   )))

(defun    (precompile-processing---ECADD_MUL_PAIRING---return-data-size)               (+    (*    ECADD_RETURN_DATA_SIZE        (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD))
                                                                                             (*    ECMUL_RETURN_DATA_SIZE        (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL))
                                                                                             (*    ECPAIRING_RETURN_DATA_SIZE    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING))))

(defun    (precompile-processing---ECADD_MUL_PAIRING---return-data-phase)              (+    (*    PHASE_ECADD_RESULT            (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD))
                                                                                             (*    PHASE_ECMUL_RESULT            (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL))
                                                                                             (*    PHASE_ECPAIRING_RESULT        (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING))))

(defun    (precompile-processing---ECADD_MUL_PAIRING---return-data-reference-size)     (+    (*    ECADD_RETURN_DATA_SIZE        (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECADD))
                                                                                             (*    ECMUL_RETURN_DATA_SIZE        (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECMUL))
                                                                                             (*    ECPAIRING_RETURN_DATA_SIZE    (precompile-processing---ECADD_MUL_PAIRING---nontrivial-ECPAIRING))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 3 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD_MUL_PAIRING---third-misc-row-peeking-flags
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---partial-return-data-copy)
                          (*    MISC_WEIGHT_MMU
                                (precompile-processing---common---OOB-r@c-nonzero))))


(defconstraint    precompile-processing---ECADD_MUL_PAIRING---setting-the-MMU-instruction---partial-return-data-copy
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (if-not-zero    (shift    misc/MMU_FLAG                           precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---partial-return-data-copy)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---ECADD_MUL_PAIRING---misc-row-offset---partial-return-data-copy  ;; offset
                                                                                    (+    1    HUB_STAMP)                                                                   ;; source ID
                                                                                    CONTEXT_NUMBER                                                                          ;; target ID
                                                                                    ;; aux_id                                                                               ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                                        ;; source offset high
                                                                                    0                                                                                       ;; source offset low
                                                                                    ;; tgt_offset_lo                                                                        ;; target offset low
                                                                                    (precompile-processing---ECADD_MUL_PAIRING---return-data-reference-size)                ;; size
                                                                                    (precompile-processing---dup-r@o)                                                       ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                                       ;; reference size
                                                                                    ;; success_bit                                                                          ;; success bit
                                                                                    ;; limb_1                                                                               ;; limb 1
                                                                                    ;; limb_2                                                                               ;; limb 2
                                                                                    ;; exo_sum                                                                              ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                                ;; phase
                                                                                    )))


;;;;;;;;;;;;;;;;;;;;;;;
;; Context-row i + 4 ;;
;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECADD_MUL_PAIRING---updating-caller-context-with-precompile-return-data
                  (:guard    (precompile-processing---ECADD_MUL_PAIRING---success-case))
                  (provide-return-data     precompile-processing---ECADD_MUL_PAIRING---context-row-offset---updating-caller-context    ;; row offset
                                           CONTEXT_NUMBER                                                                             ;; receiver context
                                           (+    1    HUB_STAMP)                                                                      ;; provider context
                                           0                                                                                          ;; rdo
                                           (precompile-processing---ECADD_MUL_PAIRING---return-data-reference-size)                   ;; rds
                                           ))

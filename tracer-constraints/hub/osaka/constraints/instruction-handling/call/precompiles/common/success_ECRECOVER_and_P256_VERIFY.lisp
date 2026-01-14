(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;   X.Y.Z.5 The  PRC/ECRECOVER success case   ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defun    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition)    (*    PEEK_AT_SCENARIO
                                                                                             (+  scenario/PRC_ECRECOVER   scenario/PRC_P256_VERIFY)
                                                                                             (scenario-shorthand---PRC---success)
                                                                                             ))


;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER and P256_VERIFY related shorthands ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (trigger-MMU-for-full-transfer)   (+   (*   (precompile-processing---common---address-recovery-success)        scenario/PRC_ECRECOVER   )
                                                 (*   (precompile-processing---common---p256-sig-verification-success)   scenario/PRC_P256_VERIFY )
                                                 ))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER and P256_VERIFY related shorthands sanity checks ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defproperty      precompile-processing---ECRECOVER-and-P256VERIFY-success---trigger-MMU-for-full-transfer-binarity-check
                  (if-not-zero   (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition)
                                 (is-binary   (trigger-MMU-for-full-transfer))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                            ;;
;;   X.Y.Z.T.U Misc row i + 2: full transfer of return data   ;;
;;                                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---ECRECOVER-and-P256VERIFY-success---2nd-misc-row---setting-module-flags
                  (:guard    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---common---2nd-misc-row---row-offset)
                          (*     MISC_WEIGHT_MMU    (trigger-MMU-for-full-transfer))))

(defconstraint    precompile-processing---ECRECOVER-and-P256VERIFY-success---2nd-misc-row---setting-MMU-instruction    (:guard    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---common---2nd-misc-row---row-offset)
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
                                                                                   EXO_SUM_WEIGHT_ECDATA                                                    ;; weighted exogenous module flag sum
                                                                                   (precompile-processing---ECRECOVER-and-P256VERIFY---mmu-phase)           ;; phase
                                                                                   )))


(defun    (precompile-processing---ECRECOVER-and-P256VERIFY---mmu-phase)    (+   (*   PHASE_ECRECOVER_RESULT     scenario/PRC_ECRECOVER   )
                                                                                 (*   PHASE_P256_VERIFY_RESULT   scenario/PRC_P256_VERIFY )
                                                                                 ))


;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER and P256_VERIFY related shorthands ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defun    (trigger-MMU-for-partial-copy)   (*   (trigger-MMU-for-full-transfer)
                                                (precompile-processing---common---OOB-r@c-nonzero)
                                                ))

;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;
;; ECRECOVER and P256_VERIFY related shorthands sanity checks ;;
;;~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~;;

(defproperty      precompile-processing---ECRECOVER-and-P256VERIFY-success---trigger-MMU-for-partial-copy-binarity-check
                  (if-not-zero   (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition)
                                 (is-binary   (trigger-MMU-for-partial-copy))))



;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                           ;;
;;   X.Y.Z.T.U Misc row i + 3: partial copy of return data   ;;
;;                                                           ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defconstraint    precompile-processing---ECRECOVER-and-P256VERIFY-success---3rd-misc-row---setting-module-flags
                  (:guard    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---common---3rd-misc-row---row-offset)
                          (*    MISC_WEIGHT_MMU
                                (trigger-MMU-for-partial-copy)
                                )))

(defconstraint    precompile-processing---ECRECOVER-and-P256VERIFY-success---3rd-misc-row---setting-MMU-instruction
                  (:guard    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---common---3rd-misc-row---row-offset)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---common---3rd-misc-row---row-offset               ;; offset
                                                                                    (+    1    HUB_STAMP)                                                    ;; source ID
                                                                                    CONTEXT_NUMBER
                                                                                    ;; aux_id                                                                   ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                            ;; source offset high
                                                                                    0                                                                           ;; source offset low
                                                                                    ;; tgt_offset_lo                                                            ;; target offset low
                                                                                    32                                                                          ;; size
                                                                                    (precompile-processing---dup-r@o)                                        ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                        ;; reference size
                                                                                    ;; success_bit                                                              ;; success bit
                                                                                    ;; limb_1                                                                   ;; limb 1
                                                                                    ;; limb_2                                                                   ;; limb 2
                                                                                    ;; exo_sum                                                                  ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                    ;; phase
                                                                                    )
                                  ))

(defconstraint    precompile-processing---ECRECOVER-and-P256VERIFY-success---updating-return-data
                  (:guard    (precompile-processing---ECRECOVER-and-P256VERIFY---success-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (provide-return-data     precompile-processing---common---context-row-success---row-offset          ;; row offset
                                           CONTEXT_NUMBER                                                             ;; receiver context
                                           (+    1    HUB_STAMP)                                                      ;; provider context
                                           0                                                                          ;; rdo
                                           (*    32    (trigger-MMU-for-full-transfer))                               ;; rds
                                           ))

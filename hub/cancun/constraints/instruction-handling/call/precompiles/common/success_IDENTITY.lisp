(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;    X.Y.Z.5 The  PRC/IDENTITY success case    ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---IDENTITY---success-precondition)    (*    PEEK_AT_SCENARIO
                                                                             scenario/PRC_IDENTITY
                                                                             (scenario-shorthand---PRC---success)))


(defconstraint    precompile-processing---IDENTITY-success---2nd-misc-row---setting-module-flags       (:guard    (precompile-processing---IDENTITY---success-precondition))
                  (eq!    (weighted-MISC-flag-sum    precompile-processing---IDENTITY---2nd-misc-row---row-offset)
                          (*    MISC_WEIGHT_MMU
                                (precompile-processing---common---OOB-extract-call-data)
                                (precompile-processing---common---OOB-r@c-nonzero)
                                )))

(defconstraint    precompile-processing---IDENTITY-success---2nd-misc-row---setting-MMU-instruction    (:guard    (precompile-processing---IDENTITY---success-precondition))
                  (if-not-zero    (shift    misc/MMU_FLAG    precompile-processing---IDENTITY---2nd-misc-row---row-offset)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---IDENTITY---2nd-misc-row---row-offset   ;; offset
                                                                                    (+    1    HUB_STAMP)                                          ;; source ID
                                                                                    CONTEXT_NUMBER                                                 ;; target ID
                                                                                    ;; aux_id                                                      ;; auxiliary ID
                                                                                    ;; src_offset_hi                                               ;; source offset high
                                                                                    0                                                              ;; source offset low
                                                                                    ;; tgt_offset_lo                                               ;; target offset low
                                                                                    (precompile-processing---dup-cds)                              ;; size
                                                                                    (precompile-processing---dup-r@o)                              ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                              ;; reference size
                                                                                    ;; success_bit                                                 ;; success bit
                                                                                    ;; limb_1                                                      ;; limb 1
                                                                                    ;; limb_2                                                      ;; limb 2
                                                                                    ;; exo_sum                                                     ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                       ;; phase
                                                                                    )
                                  ))

(defconstraint    precompile-processing---IDENTITY-success---updating-return-data                      (:guard    (precompile-processing---IDENTITY---success-precondition))
                  (provide-return-data     precompile-processing---IDENTITY---context-row-success---row-offset        ;; row offset
                                           CONTEXT_NUMBER                                                             ;; receiver context
                                           (+    1    HUB_STAMP)                                                      ;; provider context
                                           0                                                                          ;; rdo
                                           (precompile-processing---dup-cds)                                          ;; rds
                                           ))

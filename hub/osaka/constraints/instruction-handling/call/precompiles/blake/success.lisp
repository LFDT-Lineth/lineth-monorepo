(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                ;;;;
;;;;    X.Y CALL    ;;;;
;;;;                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;;    X.Y.22.5  The SUCCESS case    ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defun    (precompile-processing---BLAKE2f---success)    (*    PEEK_AT_SCENARIO
                                                               scenario/PRC_BLAKE2f
                                                               (scenario-shorthand---PRC---success)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 3 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---BLAKE2f---setting-MISC-flags---full-return-data-transfer
                  (:guard    (precompile-processing---BLAKE2f---success))
                  (eq!       (weighted-MISC-flag-sum          precompile-processing---BLAKE2f---misc-row-offset---BLAKE-return-data-full-transfer)
                             MISC_WEIGHT_MMU))


(defconstraint    precompile-processing---BLAKE2f---setting-MMU-instruction---full-return-data-transfer
                  (:guard    (precompile-processing---BLAKE2f---success))
                  (set-MMU-instruction---exo-to-ram-transplants    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-return-data-full-transfer     ;; offset
                                                                   (+    1    HUB_STAMP)                                                                   ;; source ID
                                                                   (+    1    HUB_STAMP)                                                                   ;; target ID
                                                                   ;; aux_id                                                                                  ;; auxiliary ID
                                                                   ;; src_offset_hi                                                                           ;; source offset high
                                                                   ;; src_offset_lo                                                                           ;; source offset low
                                                                   ;; tgt_offset_lo                                                                           ;; target offset low
                                                                   64                                                                                      ;; size
                                                                   ;; ref_offset                                                                              ;; reference offset
                                                                   ;; ref_size                                                                                ;; reference size
                                                                   ;; success_bit                                                                             ;; success bit
                                                                   ;; limb_1                                                                                  ;; limb 1
                                                                   ;; limb_2                                                                                  ;; limb 2
                                                                   EXO_SUM_WEIGHT_BLAKEMODEXP                                                              ;; weighted exogenous module flag sum
                                                                   PHASE_BLAKE_RESULT                                                                      ;; phase
                                                                   ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 4 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---BLAKE2f---setting-MISC-flags---partial-return-data-copy
                  (:guard    (precompile-processing---BLAKE2f---success))
                  (eq!       (weighted-MISC-flag-sum          precompile-processing---BLAKE2f---misc-row-offset---BLAKE-partial-return-data-copy)
                             (*    MISC_WEIGHT_MMU    (precompile-processing---BLAKE2f---OOB-r@c-nonzero))))

(defconstraint    precompile-processing---BLAKE2f---setting-MMU-instruction---partial-return-data-copy
                  (:guard    (precompile-processing---BLAKE2f---success))
                  (if-not-zero    (shift    misc/MMU_FLAG                           precompile-processing---BLAKE2f---misc-row-offset---BLAKE-partial-return-data-copy)
                                  (set-MMU-instruction---ram-to-ram-sans-padding    precompile-processing---BLAKE2f---misc-row-offset---BLAKE-partial-return-data-copy    ;; offset
                                                                                    (+    1    HUB_STAMP)                                                                 ;; source ID
                                                                                    CONTEXT_NUMBER                                                                        ;; target ID
                                                                                    ;; aux_id                                                                                ;; auxiliary ID
                                                                                    ;; src_offset_hi                                                                         ;; source offset high
                                                                                    0                                                                                     ;; source offset low
                                                                                    ;; tgt_offset_lo                                                                         ;; target offset low
                                                                                    64                                                                                    ;; size
                                                                                    (precompile-processing---dup-r@o)                                                     ;; reference offset
                                                                                    (precompile-processing---dup-r@c)                                                     ;; reference size
                                                                                    ;; success_bit                                                                           ;; success bit
                                                                                    ;; limb_1                                                                                ;; limb 1
                                                                                    ;; limb_2                                                                                ;; limb 2
                                                                                    ;; exo_sum                                                                               ;; weighted exogenous module flag sum
                                                                                    ;; phase                                                                                 ;; phase
                                                                                    )))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;; Miscellaneous-row i + 5 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    precompile-processing---BLAKE2f---success---squashing-caller-return-data
                  (:guard    (precompile-processing---BLAKE2f---success))
                  (provide-return-data     precompile-processing---BLAKE2f---context-row-offset---updating-caller-return-data      ;; row offset
                                           CONTEXT_NUMBER                                                                          ;; receiver context
                                           (+    1    HUB_STAMP)                                                                   ;; source IDr context
                                           0                                                                                       ;; rdo
                                           64                                                                                      ;; rds
                                           ))

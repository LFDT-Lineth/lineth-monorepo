(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.U Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;    X.U Instructions raising the LOG_FLAG   ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;    X.U.1 Supported instructions and flags   ;;
;;    X.U.2 Shorthands                         ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun (log-instruction---instruction)              stack/INSTRUCTION)
(defun (log-instruction---offset-hi)                [stack/STACK_ITEM_VALUE_HI 1])
(defun (log-instruction---offset-lo)                [stack/STACK_ITEM_VALUE_LO 1])
(defun (log-instruction---size-hi)                  [stack/STACK_ITEM_VALUE_HI 2])
(defun (log-instruction---size-lo)                  [stack/STACK_ITEM_VALUE_LO 2]) ;; ""
(defun (log-instruction---standard-hypothesis)      (*    PEEK_AT_STACK
                                                          stack/LOG_FLAG
                                                          (-    1    stack/SUX    stack/SOX)
                                                          (-    1    COUNTER_TLI)))

(defconst
  ROFF_LOG___CURRENT_CONTEXT_ROW   2
  ROFF_LOG___MISCELLANEOUS_ROW     3
  ROFF_LOG___STATICX_XCONTEXT_ROW  3
  ROFF_LOG___OTHERX_XCONTEXT_ROW   4)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.U.2 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    log-instruction---setting-the-stack-pattern                        (:guard (log-instruction---standard-hypothesis))
                  (log-stack-pattern     (- stack/INSTRUCTION EVM_INST_LOG0)
                                         [ stack/DEC_FLAG 1 ]
                                         [ stack/DEC_FLAG 2 ]
                                         [ stack/DEC_FLAG 3 ]
                                         [ stack/DEC_FLAG 4 ])) ;; ""

(defconstraint    log-instruction---allowable-exceptions                             (:guard (log-instruction---standard-hypothesis))
                  (eq! XAHOY (+ stack/STATICX
                                stack/MXPX
                                stack/OOGX)))

(defconstraint    log-instruction---setting-NSR                                      (:guard (log-instruction---standard-hypothesis))
                  (if-zero (force-bin stack/STATICX)
                           (eq! NSR (+ 2 CMC))
                           (eq! NSR    2)))

(defconstraint    log-instruction---setting-the-peeking-flags                        (:guard (log-instruction---standard-hypothesis))
                  (if-zero (force-bin stack/STATICX)
                           ;; STATICX = 0
                           (eq! NSR (+ (shift        PEEK_AT_CONTEXT          ROFF_LOG___CURRENT_CONTEXT_ROW )
                                       (shift        PEEK_AT_MISCELLANEOUS    ROFF_LOG___MISCELLANEOUS_ROW   )
                                       (* CMC (shift PEEK_AT_CONTEXT          ROFF_LOG___OTHERX_XCONTEXT_ROW ))))
                           ;; STATICX = 1
                           (eq! NSR (+ (shift        PEEK_AT_CONTEXT          ROFF_LOG___CURRENT_CONTEXT_ROW  )
                                       (shift        PEEK_AT_CONTEXT          ROFF_LOG___STATICX_XCONTEXT_ROW )))))

(defconstraint    log-instruction---justifying-static-exception                      (:guard (log-instruction---standard-hypothesis))
                  (begin
                    (read-context-data     ROFF_LOG___CURRENT_CONTEXT_ROW    CONTEXT_NUMBER)
                    (eq!   stack/STATICX
                           (shift    context/IS_STATIC    ROFF_LOG___CURRENT_CONTEXT_ROW))))

(defconstraint    log-instruction---justifying-memory-expansion-exception            (:guard (log-instruction---standard-hypothesis))
                  (if-zero (force-bin stack/STATICX)
                           (eq! stack/MXPX
                                (shift misc/MXP_MXPX ROFF_LOG___MISCELLANEOUS_ROW))))

(defconstraint    log-instruction---setting-the-gas-cost                             (:guard (log-instruction---standard-hypothesis))
                  (if-zero (force-bin (+ stack/STATICX stack/MXPX))
                           (eq! GAS_COST
                                (+ stack/STATIC_GAS
                                   (shift misc/MXP_GAS_MXP ROFF_LOG___MISCELLANEOUS_ROW)))
                           (eq! GAS_COST 0)))

(defconstraint    log-instruction---the-final-context-row                            (:guard (log-instruction---standard-hypothesis))
                  (begin
                    (if-not-zero stack/STATICX             (execution-provides-empty-return-data ROFF_LOG___STATICX_XCONTEXT_ROW))
                    (if-not-zero (+ stack/MXPX stack/OOGX) (execution-provides-empty-return-data ROFF_LOG___OTHERX_XCONTEXT_ROW))))

(defconstraint    log-instruction---setting-MISC-module-flags                        (:guard (log-instruction---standard-hypothesis))
                  (if-zero   stack/STATICX
                             (eq!    (weighted-MISC-flag-sum       ROFF_LOG___MISCELLANEOUS_ROW)
                                     (+ (* MISC_WEIGHT_MMU (trigger_MMU))
                                        MISC_WEIGHT_MXP))))

(defconstraint    log-instruction---MISC-row-setting-MXP-data                        (:guard (log-instruction---standard-hypothesis))
                  (if-zero   stack/STATICX
                             (set-MXP-instruction-type-4    ROFF_LOG___MISCELLANEOUS_ROW        ;; row offset kappa
                                                            (log-instruction---instruction)     ;; instruction
                                                            0                                   ;; bit modifying the behaviour of RETURN pricing
                                                            (log-instruction---offset-hi)       ;; offset high
                                                            (log-instruction---offset-lo)       ;; offset low
                                                            (log-instruction---size-hi)         ;; size high
                                                            (log-instruction---size-lo))))      ;; size low

(defun (trigger_MMU) (* (- 1 CONTEXT_WILL_REVERT)
                        (shift misc/MXP_MTNTOP ROFF_LOG___MISCELLANEOUS_ROW)))

(defconstraint    log-instruction---MISC-row-setting-MMU-data                        (:guard (log-instruction---standard-hypothesis))
                  (if-zero (force-bin stack/STATICX)
                           (if-not-zero (shift misc/MMU_FLAG ROFF_LOG___MISCELLANEOUS_ROW)
                                        (set-MMU-instruction---ram-to-exo-with-padding    ROFF_LOG___MISCELLANEOUS_ROW   ;; offset
                                                                                          CONTEXT_NUMBER                 ;; source ID
                                                                                          LOG_INFO_STAMP                 ;; target ID
                                                                                          0                              ;; auxiliary ID
                                                                                          ;; src_offset_hi               ;; source offset high
                                                                                          (log-instruction---offset-lo)             ;; source offset low
                                                                                          ;; tgt_offset_lo               ;; target offset low
                                                                                          (log-instruction---size-lo)               ;; size
                                                                                          ;; ref_offset                  ;; reference offset
                                                                                          (log-instruction---size-lo)               ;; reference size
                                                                                          0                              ;; success bit
                                                                                          ;; limb_1                      ;; limb 1
                                                                                          ;; limb_2                      ;; limb 2
                                                                                          EXO_SUM_WEIGHT_LOG             ;; weighted exogenous module flag sum
                                                                                          0                              ;; phase
                                                                                          ))))

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


(defun (log-inst-instruction)        stack/INSTRUCTION)
(defun (log-inst-offset-hi)        [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun (log-inst-offset-lo)        [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun (log-inst-size-hi)          [ stack/STACK_ITEM_VALUE_HI 2 ])
(defun (log-inst-size-lo)          [ stack/STACK_ITEM_VALUE_LO 2 ])

(defun (log-inst-standard-hypothesis) (* PEEK_AT_STACK
                                         stack/LOG_FLAG
                                         (- 1 stack/SUX    stack/SOX)
                                         (- 1 COUNTER_TLI           )))

(defconst
  log-context-row-offset                 2
  log-misc-row-offset                    3
  log-staticx-context-row-offset         3
  log-other-x-context-row-offset         4
  )


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.U.2 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint log-inst-setting-the-stack-pattern                        (:guard (log-inst-standard-hypothesis))
               (log-stack-pattern     (- stack/INSTRUCTION EVM_INST_LOG0)
                                      [ stack/DEC_FLAG 1 ]
                                      [ stack/DEC_FLAG 2 ]
                                      [ stack/DEC_FLAG 3 ]
                                      [ stack/DEC_FLAG 4 ]))

(defconstraint log-inst-allowable-exceptions                             (:guard (log-inst-standard-hypothesis))         ;; TODO: solo debug constraint plz
               (begin (vanishes! 0)
                      (debug (eq! XAHOY (+ stack/STATICX
                                           stack/MXPX
                                           stack/OOGX)))))

(defconstraint log-inst-setting-NSR                                      (:guard (log-inst-standard-hypothesis))
               (if-zero (force-bin stack/STATICX)
                        (eq! NSR (+ 2 NSR))
                        (eq! NSR    2)))

(defconstraint log-inst-setting-the-peeking-flags                        (:guard (log-inst-standard-hypothesis))
               (if-zero (force-bin stack/STATICX)
                        ;; STATICX = 0
                        (eq! NSR (+ (shift        PEEK_AT_CONTEXT          log-context-row-offset        )
                                    (shift        PEEK_AT_MISCELLANEOUS    log-misc-row-offset           )
                                    (* CMC (shift PEEK_AT_MISCELLANEOUS    log-other-x-context-row-offset))))
                        ;; STATICX = 1
                        (eq! NSR (+ (shift        PEEK_AT_CONTEXT          log-context-row-offset        )
                                    (shift        PEEK_AT_MISCELLANEOUS    log-staticx-context-row-offset)))))

(defconstraint log-inst-justifying-static-exception                      (:guard (log-inst-standard-hypothesis))
               (begin
                 (read-context-data 2 CONTEXT_NUMBER )
                 (eq! stack/STATICX context/IS_STATIC)))

(defconstraint log-inst-justifying-memory-expansion-exception            (:guard (log-inst-standard-hypothesis))
               (if-zero (force-bin stack/STATICX)
                        (eq! stack/MXPX
                             (shift misc/MXP_MXPX log-misc-row-offset))))

(defconstraint log-inst-setting-the-gas-cost                             (:guard (log-inst-standard-hypothesis))
               (if-zero (force-bin (+ stack/STATICX stack/MXPX))
                        (eq! GAS_COST
                             (+ stack/STATIC_GAS
                                (shift misc/MXP_GAS_MXP log-misc-row-offset)))
                        (eq! GAS_COST 0)))

(defconstraint log-inst-the-final-context-row                            (:guard (log-inst-standard-hypothesis))
               (begin
                 (if-not-zero stack/STATICX             (execution-provides-empty-return-data log-staticx-context-row-offset))
                 (if-not-zero (+ stack/MXPX stack/OOGX) (execution-provides-empty-return-data log-other-x-context-row-offset))))

(defconstraint log-inst-setting-MISC-module-flags                        (:guard (log-inst-standard-hypothesis))
               (eq! (weighted-MISC-flag-sum       log-misc-row-offset)
                    (+ (* MISC_WEIGHT_MMU (trigger_MMU))
                       MISC_WEIGHT_MXP)))

(defconstraint log-inst-MISC-row-setting-MXP-data                        (:guard (log-inst-standard-hypothesis))
               (set-MXP-instruction-type-4 log-misc-row-offset        ;; row offset kappa
                                           (log-inst-instruction)     ;; instruction
                                           0                          ;; bit modifying the behaviour of RETURN pricing
                                           (log-inst-offset-hi)       ;; offset high
                                           (log-inst-offset-lo)       ;; offset low
                                           (log-inst-size-hi)         ;; size high
                                           (log-inst-size-lo)))       ;; size low

(defun (trigger_MMU) (* (- 1 CONTEXT_WILL_REVERT)
                        (shift misc/MXP_MTNTOP log-misc-row-offset)))

(defconstraint log-inst-MISC-row-setting-MMU-data                        (:guard (log-inst-standard-hypothesis))
               (if-zero (force-bin stack/STATICX)
                        (if-not-zero (shift misc/MMU_FLAG log-misc-row-offset)
                                     (set-MMU-instruction-ram-to-exo-with-padding    log-misc-row-offset            ;; offset
                                                                                     CONTEXT_NUMBER                 ;; source ID
                                                                                     LOG_INFO_STAMP                 ;; target ID
                                                                                     0                              ;; auxiliary ID
                                                                                     ;; src_offset_hi               ;; source offset high
                                                                                     (log-inst-offset-lo)             ;; source offset low
                                                                                     ;; tgt_offset_lo               ;; target offset low
                                                                                     (log-inst-size-lo)               ;; size
                                                                                     ;; ref_offset                  ;; reference offset
                                                                                     (log-inst-size-lo)               ;; reference size
                                                                                     0                              ;; success bit
                                                                                     ;; limb_1                      ;; limb 1
                                                                                     ;; limb_2                      ;; limb 2
                                                                                     EXO_SUM_WEIGHT_LOG             ;; weighted exogenous module flag sum
                                                                                     0                              ;; phase
                                                                                     ))))

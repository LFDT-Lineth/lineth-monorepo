(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.5.27 Machine state instructions   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconst
  ROFF_MACHINESTATE___NOT_MSIZE___XCONTEXT_ROW    1

  ROFF_MACHINESTATE___MSIZE___MISC_ROW            1
  ROFF_MACHINESTATE___MSIZE___XCONTEXT_ROW        2)


(defun    (machine-state-instruction---is-PC)          [ stack/DEC_FLAG 1 ])
(defun    (machine-state-instruction---is-MSIZE)       [ stack/DEC_FLAG 2 ])
(defun    (machine-state-instruction---is-GAS)         [ stack/DEC_FLAG 3 ])
(defun    (machine-state-instruction---is-JUMPDEST)    [ stack/DEC_FLAG 4 ])

(defun    (machine-state-instruction---result-hi)      [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (machine-state-instruction---result-lo)      [ stack/STACK_ITEM_VALUE_LO 4 ]) ;; ""

(defun (machine-state-instruction---isnt-MSIZE)        (+ (machine-state-instruction---is-PC)
                                                          (machine-state-instruction---is-GAS)
                                                          (machine-state-instruction---is-JUMPDEST)))

(defun (machine-state-instruction---isnt-JUMPDEST)     (+ (machine-state-instruction---is-PC)
                                                          (machine-state-instruction---is-MSIZE)
                                                          (machine-state-instruction---is-GAS)))

(defun (machine-state-instruction---no-stack-exception)    (* PEEK_AT_STACK
                                                              stack/MACHINE_STATE_FLAG
                                                              (- 1 stack/SUX stack/SOX)))

(defconstraint machine-state-instruction---setting-the-stack-pattern (:guard (machine-state-instruction---no-stack-exception))
               (begin
                 (if-not-zero   (machine-state-instruction---isnt-JUMPDEST)   (stack-pattern-0-1))
                 (if-not-zero   (machine-state-instruction---is-JUMPDEST)     (stack-pattern-0-0))))

(defconstraint machine-state-instruction---excluding-certain-exceptions (:guard (machine-state-instruction---no-stack-exception))
               (eq!   XAHOY   stack/OOGX))

(defconstraint machine-state-instruction---setting-NSR               (:guard (machine-state-instruction---no-stack-exception))
               (eq!    NSR    (+ (machine-state-instruction---is-MSIZE) CMC)))

(defconstraint machine-state-instruction---setting-peeking-flags     (:guard (machine-state-instruction---no-stack-exception))
               (begin
                 (if-not-zero   (machine-state-instruction---isnt-MSIZE)   (eq! NSR (*   (shift      PEEK_AT_CONTEXT          ROFF_MACHINESTATE___NOT_MSIZE___XCONTEXT_ROW)  CMC)))
                 (if-not-zero   (machine-state-instruction---is-MSIZE)     (eq! NSR (+   (shift      PEEK_AT_MISCELLANEOUS    ROFF_MACHINESTATE___MSIZE___MISC_ROW        )
                                                                                         (* (shift   PEEK_AT_CONTEXT          ROFF_MACHINESTATE___MSIZE___XCONTEXT_ROW    )  CMC))))))

(defconstraint machine-state-instruction---setting-miscellaneous-row (:guard (machine-state-instruction---no-stack-exception))
               (if-not-zero    (machine-state-instruction---is-MSIZE)
                               (begin
                                 (eq! (weighted-MISC-flag-sum    ROFF_MACHINESTATE___MSIZE___MISC_ROW) MISC_WEIGHT_MXP)
                                 (set-MXP-instruction-type-1     ROFF_MACHINESTATE___MSIZE___MISC_ROW))))

(defconstraint machine-state-instruction---setting-gas-cost
               (:guard (machine-state-instruction---no-stack-exception))
               (eq! GAS_COST stack/STATIC_GAS))

(defconstraint machine-state-instruction---all-instructions-produce-zero-high-part
               (:guard (machine-state-instruction---no-stack-exception))
               (if-zero    XAHOY
                           (vanishes!     (machine-state-instruction---result-hi))))

(defconstraint machine-state-instruction---setting-stack-value---PC-case
               (:guard (machine-state-instruction---no-stack-exception))
               (if-zero    XAHOY
                           (if-not-zero   (machine-state-instruction---is-PC)
                                          (eq! (machine-state-instruction---result-lo)   PROGRAM_COUNTER))))

(defconstraint machine-state-instruction---setting-stack-value---MSIZE-case
               (:guard (machine-state-instruction---no-stack-exception))
               (if-zero    XAHOY
                           (if-not-zero   (machine-state-instruction---is-MSIZE)
                                          (eq! (machine-state-instruction---result-lo)   (shift    (*   misc/MXP_WORDS   WORD_SIZE)   ROFF_MACHINESTATE___MSIZE___MISC_ROW)))))

(defconstraint machine-state-instruction---setting-stack-value---GAS-case
               (:guard (machine-state-instruction---no-stack-exception))
               (if-zero    XAHOY
                           (if-not-zero   (machine-state-instruction---is-GAS)
                                          (eq! (machine-state-instruction---result-lo)   GAS_NEXT))))

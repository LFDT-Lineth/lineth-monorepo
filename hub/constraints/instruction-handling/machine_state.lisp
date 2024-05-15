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

(defun (machine-state-instruction-no-stack-exception)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK
     stack/MACHINE_STATE_FLAG
     (- 1 stack/SUX stack/SOX)))

(defun (machine-state-DEC_FLAG-sum) (+ [ stack/DEC_FLAG 1 ]
                                       [ stack/DEC_FLAG 2 ]
                                       [ stack/DEC_FLAG 3 ]))

(defconstraint machine-state-setting-the-stack-pattern (:guard (machine-state-instruction-no-stack-exception))
               (begin
                 (if-not-zero (machine-state-DEC_FLAG-sum) (stack-pattern-0-1))
                 (if-not-zero [ stack/DEC_FLAG 4 ]         (stack-pattern-0-0))))

;; TODO: debug shouldn't require another 'non-debug' constraint
(defconstraint machine-state-excluding-certain-exceptions (:guard (machine-state-instruction-no-stack-exception))
               (begin
                 (vanishes! 0)
                 (debug (eq! XAHOY stack/OOGX))))

(defconstraint machine-state-setting-NSR               (:guard (machine-state-instruction-no-stack-exception))
               (eq! NSR (+ stack/MXP_FLAG CMC)))

(defconstraint machine-state-setting-peeking-flags     (:guard (machine-state-instruction-no-stack-exception))
               (if-zero (force-bin stack/MXP_FLAG)
                        (eq! NSR (* CMC (next PEEK_AT_CONTEXT)))
                        (eq! NSR (+ (shift PEEK_AT_CONTEXT 1)
                                    (* CMC (shift PEEK_AT_CONTEXT 2))))))

(defconstraint machine-state-setting-miscellaneous-row (:guard (machine-state-instruction-no-stack-exception))
               (if-not-zero stack/MXP_FLAG
                            (eq! (weighted-MISC-flag-sum    1) MISC_WEIGHT_MXP)
                            (set-MXP-instruction-type-1     1)))

(defconstraint machine-state-setting-gas-cost          (:guard (machine-state-instruction-no-stack-exception))
               (eq! GAS_COST stack/STATIC_GAS))

(defconstraint machine-state-setting-stack-value       (:guard (machine-state-instruction-no-stack-exception))
               (begin
                 (vanishes! [ stack/STACK_ITEM_VALUE_HI 4 ])
                 (if-not-zero [ stack/DEC_FLAG 1] (eq! [ stack/STACK_ITEM_VALUE_LO 4] PROGRAM_COUNTER))
                 (if-not-zero [ stack/DEC_FLAG 2] (eq! [ stack/STACK_ITEM_VALUE_LO 4] (next misc/MXP_WORDS)))
                 (if-not-zero [ stack/DEC_FLAG 3] (eq! [ stack/STACK_ITEM_VALUE_LO 4] GAS_NEXT))))

;; (defconstraint machine-state- (:guard (machine-state-instruction-no-stack-exception))

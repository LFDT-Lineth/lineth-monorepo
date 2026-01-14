(module hub)

(defun (swap-no-stack-exceptions)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK
     stack/SWAP_FLAG
     (- 1 stack/SUX stack/SOX)))

(defconstraint swap-stack-pattern (:guard (swap-no-stack-exceptions))
               (swap-stack-pattern (- stack/INSTRUCTION (- EVM_INST_SWAP1 1))))

(defconstraint swap-setting-NSR (:guard (swap-no-stack-exceptions))
               (eq! NSR CMC))

(defconstraint swap-setting-peeking-flags (:guard (swap-no-stack-exceptions))
               (eq! NSR (* CMC (next PEEK_AT_CONTEXT))))

(defconstraint swap-setting-gas-costs (:guard (swap-no-stack-exceptions))
               (eq! GAS_COST stack/STATIC_GAS))

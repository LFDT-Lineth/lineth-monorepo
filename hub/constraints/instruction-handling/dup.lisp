(module hub)

(defun (dup-no-stack-exceptions)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK
     stack/DUP_FLAG
     (- 1 stack/SUX stack/SOX)))

(defconstraint dup-stack-pattern (:guard (dup-no-stack-exceptions))
               (dup-stack-pattern (- stack/INSTRUCTION EVM_INST_DUP1)))

(defconstraint dup-setting-NSR (:guard (dup-no-stack-exceptions))
               (eq! NSR CMC))

(defconstraint dup-setting-peeking-flags (:guard (dup-no-stack-exceptions))
               (eq! NSR (* CMC (next PEEK_AT_CONTEXT))))

(defconstraint dup-setting-gas-costs (:guard (dup-no-stack-exceptions))
               (eq! GAS_COST stack/STATIC_GAS))

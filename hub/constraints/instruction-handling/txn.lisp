(module hub)

(defun (txn-no-stack-exceptions)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK stack/TXN_FLAG (- 1 stack/SUX stack/SOX)))

(defconstraint txn-stack-pattern (:guard (txn-no-stack-exceptions))
  (stack-pattern-0-1))

(defconstraint txn-setting-NSR (:guard (txn-no-stack-exceptions))
  (eq! NSR (+ 1 CMC)))

(defconstraint txn-setting-peeking-flags (:guard (txn-no-stack-exceptions))
  (eq! (+ (shift PEEK_AT_TRANSACTION 1)
          (* (shift PEEK_AT_CONTEXT 2) CMC))
       NSR))

(defconstraint txn-setting-gas-cost (:guard (txn-no-stack-exceptions))
  (eq! GAS_COST stack/STATIC_GAS))

(defconstraint txn-setting-stack-values (:guard (txn-no-stack-exceptions))
  (if-zero (force-bin [stack/DEC_FLAG 1])
           (begin (eq! [stack/STACK_ITEM_VALUE_HI 4] (next transaction/FROM_ADDRESS_HI))
                  (eq! [stack/STACK_ITEM_VALUE_LO 4] (next transaction/FROM_ADDRESS_LO)))
           (begin (vanishes! [stack/STACK_ITEM_VALUE_HI 4])
                  (eq! [stack/STACK_ITEM_VALUE_LO 4] (next transaction/GAS_PRICE)))))

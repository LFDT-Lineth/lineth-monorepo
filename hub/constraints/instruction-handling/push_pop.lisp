(module hub)

(defun (push-pop-no-stack-exceptions)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK stack/PUSHPOP_FLAG (- 1 stack/SUX stack/SOX)))

(defconstraint push-pop-stack-pattern (:guard (push-pop-no-stack-exceptions))
               (begin (if-not-zero (- 1 [stack/DEC_FLAG 1])
                                   (stack-pattern-1-0))
                      (if-not-zero [stack/DEC_FLAG 1]
                                   (stack-pattern-0-1))))

(defconstraint push-pop-setting-NSR (:guard (push-pop-no-stack-exceptions))
  (eq! NSR CMC))

;; TODO: make debug
(defconstraint push-pop-setting-peeking-flags (:guard (push-pop-no-stack-exceptions))
  ;; (debug (eq! NSR (* CMC (next PEEK_AT_CONTEXT)))))
  (eq! NSR
       (* CMC (next PEEK_AT_CONTEXT))))

(defconstraint push-pop-setting-gas-costs (:guard (push-pop-no-stack-exceptions))
  (eq! GAS_COST stack/STATIC_GAS))

(defconstraint push-pop-setting-stack-values (:guard (push-pop-no-stack-exceptions))
               (if-not-zero [stack/DEC_FLAG 1]
                            (begin (eq! [stack/STACK_ITEM_VALUE_HI 4] stack/PUSH_VALUE_HI)
                                   (eq! [stack/STACK_ITEM_VALUE_LO 4] stack/PUSH_VALUE_LO))))

(defconstraint push-pop-setting-PC_NEW (:guard (push-pop-no-stack-exceptions))
               (if-zero (force-bin [ stack/DEC_FLAG 1 ])
                        (eq! PC_NEW (+ 1 PC))
                        (eq! PC_NEW (+ 1 PC 1 (- stack/INSTRUCTION EVM_INST_PUSH1)))))

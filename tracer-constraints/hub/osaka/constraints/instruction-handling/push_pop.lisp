(module hub)

(defun    (push-pop-instruction---standard-hypothesis)   (force-bin (* PEEK_AT_STACK stack/PUSHPOP_FLAG (- 1 stack/SUX stack/SOX))))
(defun    (push-pop-instruction---result-hi)             [ stack/STACK_ITEM_VALUE_HI  4 ])
(defun    (push-pop-instruction---result-lo)             [ stack/STACK_ITEM_VALUE_LO  4 ])
(defun    (push-pop-instruction---is-POP)                [ stack/DEC_FLAG             1 ])
(defun    (push-pop-instruction---is-PUSH)               [ stack/DEC_FLAG             2 ])
(defun    (push-pop-instruction---is-PUSH-ZERO)          [ stack/DEC_FLAG             3 ])

(defconstraint    push-pop-instruction---setting-the-stack-pattern---POP-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (push-pop-instruction---is-POP)
                               (stack-pattern-1-0)))

(defconstraint    push-pop-instruction---setting-the-stack-pattern---PUSH-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (force-bin (+ (push-pop-instruction---is-PUSH) (push-pop-instruction---is-PUSH-ZERO)))
                               (stack-pattern-0-1)))

(defconstraint    push-pop-instruction---setting-NSR
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (eq! NSR CMC))

;; this could be debug ...
(defconstraint    push-pop-instruction---setting-the-peeking-flags
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (eq! NSR
                       (* CMC (next PEEK_AT_CONTEXT))))

(defconstraint    push-pop-instruction---setting-gas-costs
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (eq! GAS_COST stack/STATIC_GAS))

(defconstraint    push-pop-instruction---setting-stack-values---PUSH-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (push-pop-instruction---is-PUSH)
                               (begin (eq! (push-pop-instruction---result-hi) stack/PUSH_VALUE_HI)
                                      (eq! (push-pop-instruction---result-lo) stack/PUSH_VALUE_LO))))

(defconstraint    push-pop-instruction---setting-stack-values---PUSH0-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (push-pop-instruction---is-PUSH-ZERO)
                               (begin (vanishes! (push-pop-instruction---result-hi))
                                      (vanishes! (push-pop-instruction---result-lo)))))

(defconstraint    push-pop-instruction---setting-PC_NEW---POP-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (push-pop-instruction---is-POP)
                               (eq! PC_NEW (+ 1 PC))))

(defconstraint    push-pop-instruction---setting-PC_NEW---PUSH-case
                  (:guard (push-pop-instruction---standard-hypothesis))
                  (if-not-zero (force-bin (+ (push-pop-instruction---is-PUSH) (push-pop-instruction---is-PUSH-ZERO)))
                               (eq! PC_NEW (+ 1 PC (- stack/INSTRUCTION EVM_INST_PUSH0)))))

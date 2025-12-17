(module rlptxn)

(defun (is-first-row-of-transaction) (force-bin (* (- 1 (prev IS_RLP_PREFIX)) IS_RLP_PREFIX))) 

(defconstraint user-tx-num-vanishes-in-padding (:domain {0}) (vanishes! USER_TXN_NUMBER))

(defcomputedcolumn (USER_TXN_NUMBER :i16 :fwd) (+ (prev USER_TXN_NUMBER) (is-first-row-of-transaction)))
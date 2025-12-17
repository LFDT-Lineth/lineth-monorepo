(module rlptxn)

(defun (transaction-constant col) 
  (if-not (== USER_TXN_NUMBER (+ 1 (prev USER_TXN_NUMBER)))
          (remained-constant! col)))

(defconstraint transaction-constancies ()
               (begin
                 (transaction-constant CFI)
                 (transaction-constant REPLAY_PROTECTION)
                 (transaction-constant Y_PARITY)
                 ))

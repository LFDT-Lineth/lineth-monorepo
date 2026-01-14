(module rlptxn)

(defconstraint finalization (:domain {-1})
    (if-not-zero USER_TXN_NUMBER
        (begin 
        (eq! PHASE_END 1)
        (eq! IS_S      1))))
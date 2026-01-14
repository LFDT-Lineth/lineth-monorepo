(module rlptxn)

;;note: :i16 is not enough for ref tests

(defcomputedcolumn (INDEX_LT :i32 :fwd) 
    (if-not-zero (is-first-row-of-transaction)
        ;; initialization
        0
        ;; update
        (+ (prev INDEX_LT) (* (prev LC) (prev LT)))
        ))

(defcomputedcolumn (INDEX_LX :i32 :fwd) 
    (if-not-zero (is-first-row-of-transaction)
        ;; initialization
        0
        ;; update
        (+ (prev INDEX_LX) (* (prev LC) (prev LX)))
        ))
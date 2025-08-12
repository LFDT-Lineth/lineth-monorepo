(module rlptxn)

(defcomputedcolumn (INDEX_LT :i16 :fwd) 
    (if-not-zero (is-first-row-of-transaction)
        ;; initialization
        0
        ;; update
        (+ (prev INDEX_LT) (* (prev LC) (prev LT)))
        ))

(defcomputedcolumn (INDEX_LX :i16 :fwd) 
    (if-not-zero (is-first-row-of-transaction)
        ;; initialization
        0
        ;; update
        (+ (prev INDEX_LX) (* (prev LC) (prev LX)))
        ))
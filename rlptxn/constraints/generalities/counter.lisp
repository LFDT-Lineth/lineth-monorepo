(module rlptxn)

(defconstraint counter-constancy-of-ct-max () (counter-constant CT_MAX CT))

(defconstraint automatic-vanishing-of-counters-outside-of-computation-rows ()
    (if-zero CMP 
        (begin 
        (vanishes! CT)
        (vanishes! CT_MAX))))

(defconstraint ct-loop ()
    (if (== CT CT_MAX)
        (vanishes! (next CT))
        (will-inc! CT 1)))

(defcomputedcolumn (DONE :binary)
    (if (== CT CT_MAX)
        (phase-flag-sum)
        0))

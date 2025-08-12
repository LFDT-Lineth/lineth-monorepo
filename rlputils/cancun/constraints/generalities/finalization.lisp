(module rlputils)

(defconstraint finalization (:domain {-1})
    (if-not-zero IOMF 
        (begin
        (eq! COMPT 1)
        (eq! CT    CT_MAX))))
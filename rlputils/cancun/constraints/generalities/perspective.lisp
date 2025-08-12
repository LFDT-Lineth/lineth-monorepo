(module rlputils)

(defconstraint iomf-is-persp-flag-sum ()
    (eq! IOMF (+ MACRO COMPT)))

(defproperty first-non-padding-row-id-MACRO
    (if-zero IOMF (eq! (next IOMF) (next MACRO))))

(defconstraint persp-evolution ()
    (if (== CT CT_MAX)
        (begin 
        (if-not-zero MACRO (will-eq! COMPT 1))
        (if-not-zero COMPT (will-eq! MACRO 1)))))
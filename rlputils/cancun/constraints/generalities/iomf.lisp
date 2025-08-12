(module rlputils)

(defconstraint initialization (:domain {0})  (vanishes! IOMF))

(defconstraint iomf-increments () (increment-by-at-most-one IOMF))
(module blsdata)

;; TODO: disable for Prague
(defconstraint cancun-restriction ()
    (eq! (flag_sum) (is_point_evaluation)))

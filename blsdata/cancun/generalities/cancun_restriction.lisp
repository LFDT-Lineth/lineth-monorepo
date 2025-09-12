(module blsdata)

(defconstraint cancun-restriction ()
    (eq! (flag_sum) (is_point_evaluation)))
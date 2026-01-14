(module blsdata)

(defun (small_point_is_nontrivial)
    (- 1 IS_INFINITY))

(defun (large_point_is_nontrivial)
    (- 1 (next IS_INFINITY)))

(defconstraint nontrivial-pop-bit-is-only-relevant-in-pairing-check ()
    (if-zero DATA_BLS_PAIRING_CHECK_FLAG
        (vanishes! NONTRIVIAL_POP_BIT)))

(defconstraint set-non-trivial-pop-bit ()
    (if-not-zero DATA_BLS_PAIRING_CHECK_FLAG
        (if-not-zero (will_switch_from_first_to_second)
            (eq! NONTRIVIAL_POP_BIT (* (small_point_is_nontrivial) (large_point_is_nontrivial))))))


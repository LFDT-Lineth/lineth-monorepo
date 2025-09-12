(module blsdata)

(defconstraint non-trivial-pop-acc-is-only-relevant-in-pairing-check ()
    (if-zero DATA_BLS_PAIRING_CHECK_FLAG
        (vanishes! NONTRIVIAL_POP_ACC)))

(defconstraint set-non-trivial-pop-acc ()
    (if-zero (prev DATA_BLS_PAIRING_CHECK_FLAG)
        (if-not-zero DATA_BLS_PAIRING_CHECK_FLAG
            (eq! NONTRIVIAL_POP_ACC NONTRIVIAL_POP_BIT))))
            
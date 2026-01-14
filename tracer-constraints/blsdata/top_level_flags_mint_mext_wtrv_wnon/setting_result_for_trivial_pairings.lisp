(module blsdata)

(defconstraint set-pairing-check-result-when-trivial-pairings ()
  (let ((pairing_result_hi (next LIMB))
        (pairing_result_lo (shift LIMB 2)))
        (if-not-zero (is_pairing_check)
            (if-not-zero (transition_to_result)
                (if-not-zero WTRV
                            (begin (vanishes! pairing_result_hi)
                                   (eq! pairing_result_lo 1)))))))
(module blsdata)

(defun (malformed_data)
    (+ MINT MEXT))

(defun (wellformed_data)
    (+ WTRV WNON))

(defun (case_data_sum)
    (+ (malformed_data)
       (wellformed_data)))

(defconstraint case-data-sum-equal-to-flag-sum ()
    (eq! (case_data_sum) (flag_sum)))

(defconstraint map-fp-to-g1-cannot-fail-externally ()
    (if-not-zero (is_map_fp_to_g1)
        (vanishes! MEXT)))

(defconstraint map-fp2-to-g2-cannot-fail-externally ()
    (if-not-zero (is_map_fp2_to_g2)
        (vanishes! MEXT)))

(defconstraint only-pairing-check-can-be-trivial ()
    (if-zero (is_pairing_check)
        (vanishes! WTRV)))

(defconstraint pairing-check-setting-non-trivial ()
    (if-not-zero (is_pairing_check)
        (if-not-zero (transition_to_result)
            (begin 
                (debug (if-not-zero (malformed_data)
                    (vanishes! WNON)))
                (if-not-zero (wellformed_data)
                    (eq! WNON NONTRIVIAL_POP_ACC))))))

(defconstraint setting-mint-bit-along-g1-msm-scalar ()
    (if-not-zero DATA_BLS_G1_MSM_FLAG
        (if-not-zero IS_SECOND_INPUT
            (vanishes! MINT_BIT))))

(defconstraint setting-mint-bit-along-g2-msm-scalar ()
    (if-not-zero DATA_BLS_G2_MSM_FLAG
        (if-not-zero IS_SECOND_INPUT
            (vanishes! MINT_BIT))))
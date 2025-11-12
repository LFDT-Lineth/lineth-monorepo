(module blsdata)

(defun (address_sum)
    (+ (* 10 (is_point_evaluation))
       (* 11 (is_g1_add))
       (* 12 (is_g1_msm))
       (* 13 (is_g2_add))
       (* 14 (is_g2_msm))
       (* 15 (is_pairing_check))
       (* 16 (is_map_fp_to_g1))
       (* 17 (is_map_fp2_to_g2))))

(defconstraint stamp-constancy-address-sum ()
    (stamp-constancy STAMP (address_sum)))

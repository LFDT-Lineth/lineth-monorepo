(module blsdata)

(defun (is_point_evaluation)
    (+ DATA_POINT_EVALUATION_FLAG RSLT_POINT_EVALUATION_FLAG))

(defun (is_g1_add)
    (+ DATA_BLS_G1_ADD_FLAG RSLT_BLS_G1_ADD_FLAG))

(defun (is_g1_msm)
    (+ DATA_BLS_G1_MSM_FLAG RSLT_BLS_G1_MSM_FLAG))

(defun (is_g2_add)
    (+ DATA_BLS_G2_ADD_FLAG RSLT_BLS_G2_ADD_FLAG))

(defun (is_g2_msm)
    (+ DATA_BLS_G2_MSM_FLAG RSLT_BLS_G2_MSM_FLAG))

(defun (is_pairing_check)
    (+ DATA_BLS_PAIRING_CHECK_FLAG RSLT_BLS_PAIRING_CHECK_FLAG))

(defun (is_map_fp_to_g1)
    (+ DATA_BLS_MAP_FP_TO_G1_FLAG RSLT_BLS_MAP_FP_TO_G1_FLAG))

(defun (is_map_fp2_to_g2)
    (+ DATA_BLS_MAP_FP2_TO_G2_FLAG RSLT_BLS_MAP_FP2_TO_G2_FLAG))

(defun (flag_sum)
    (+ (is_point_evaluation)
       (is_g1_add)
       (is_g1_msm)
       (is_g2_add)
       (is_g2_msm)
       (is_pairing_check)
       (is_map_fp_to_g1)
       (is_map_fp2_to_g2)))

(defconstraint first-row-sanity-check (:domain {0})
    (debug (vanishes! (flag_sum))))

(defconstraint non-decreasing-sanity-check ()
    (debug (if-not-zero (flag_sum) (next (eq! (flag_sum) 1)))))

(defconstraint flag-sum-when-stamp-is-zero ()
    (if-zero STAMP (vanishes! (flag_sum))))

(defconstraint flag-sum-when-stamp-is-not-zero ()
    (if-not-zero STAMP (eq! (flag_sum) 1)))
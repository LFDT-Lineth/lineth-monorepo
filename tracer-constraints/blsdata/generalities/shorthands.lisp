(module blsdata)

(defun (is_data)
    (+  DATA_POINT_EVALUATION_FLAG
        DATA_BLS_G1_ADD_FLAG
        DATA_BLS_G1_MSM_FLAG
        DATA_BLS_G2_ADD_FLAG
        DATA_BLS_G2_MSM_FLAG
        DATA_BLS_PAIRING_CHECK_FLAG
        DATA_BLS_MAP_FP_TO_G1_FLAG
        DATA_BLS_MAP_FP2_TO_G2_FLAG))

(defun (is_result)
    (+  RSLT_POINT_EVALUATION_FLAG
        RSLT_BLS_G1_ADD_FLAG
        RSLT_BLS_G1_MSM_FLAG
        RSLT_BLS_G2_ADD_FLAG
        RSLT_BLS_G2_MSM_FLAG
        RSLT_BLS_PAIRING_CHECK_FLAG
        RSLT_BLS_MAP_FP_TO_G1_FLAG
        RSLT_BLS_MAP_FP2_TO_G2_FLAG))

(defun (transition_to_data)
    (* (- 1 (is_data)) (next (is_data))))

(defun (transition_to_result)
    (* (- 1 (is_result)) (next (is_result))))

(defun (transition_bit)
    (+ (transition_to_data) (transition_to_result)))
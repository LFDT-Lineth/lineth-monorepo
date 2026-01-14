(module blsdata)

(defun (cs_G1MT_for_g1_msm)
    (* DATA_BLS_G1_MSM_FLAG IS_FIRST_INPUT MEXT_BIT))

(defun (cs_G2MT_for_g2_msm)
    (* DATA_BLS_G2_MSM_FLAG IS_FIRST_INPUT MEXT_BIT))

(defun (cs_G1MT_for_pairing_malformed)
    (* DATA_BLS_PAIRING_CHECK_FLAG IS_FIRST_INPUT MEXT_BIT))

(defun (cs_G2MT_for_pairing_malformed)
    (* DATA_BLS_PAIRING_CHECK_FLAG IS_SECOND_INPUT MEXT_BIT))

(defun (cs_G1MT_for_pairing_wellformed)
    (* DATA_BLS_PAIRING_CHECK_FLAG IS_FIRST_INPUT (- 1 NONTRIVIAL_POP_BIT) (- 1 IS_INFINITY) (wellformed_data)))

(defun (cs_G2MT_for_pairing_wellformed)
    (* DATA_BLS_PAIRING_CHECK_FLAG IS_SECOND_INPUT (- 1 NONTRIVIAL_POP_BIT) (- 1 IS_INFINITY) (wellformed_data)))  

(defun (is_nontrivial_pairing_data_or_result)
    (+ (* DATA_BLS_PAIRING_CHECK_FLAG NONTRIVIAL_POP_BIT) RSLT_BLS_PAIRING_CHECK_FLAG))  

;; Circuit selector column definitions

(defcomputedcolumn (CIRCUIT_SELECTOR_C1_MEMBERSHIP :binary@prove) 
    (* MEXT_BIT DATA_BLS_G1_ADD_FLAG))

(defcomputedcolumn (CIRCUIT_SELECTOR_C2_MEMBERSHIP :binary@prove) 
    (* MEXT_BIT DATA_BLS_G2_MSM_FLAG))

(defcomputedcolumn (CIRCUIT_SELECTOR_G1_MEMBERSHIP :binary@prove) 
    (+ (cs_G1MT_for_g1_msm)
       (cs_G1MT_for_pairing_malformed)
       (cs_G1MT_for_pairing_wellformed)))

(defcomputedcolumn (CIRCUIT_SELECTOR_G2_MEMBERSHIP :binary@prove)
    (+ (cs_G2MT_for_g2_msm)
       (cs_G2MT_for_pairing_malformed)
       (cs_G2MT_for_pairing_wellformed)))

(defcomputedcolumn (CIRCUIT_SELECTOR_POINT_EVALUATION :binary@prove) 
    (* WNON (is_point_evaluation)))

(defcomputedcolumn (CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE :binary@prove) 
    (* MEXT (is_point_evaluation)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_G1_ADD :binary@prove) 
    (* WNON (is_g1_add)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_G1_MSM :binary@prove) 
    (* WNON (is_g1_msm)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_G2_ADD :binary@prove) 
    (* WNON (is_g2_add)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_G2_MSM :binary@prove) 
    (* WNON (is_g2_msm)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_PAIRING_CHECK :binary@prove) 
    (* WNON (is_nontrivial_pairing_data_or_result)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_MAP_FP_TO_G1 :binary@prove) 
    (* WNON (is_map_fp_to_g1)))

(defcomputedcolumn (CIRCUIT_SELECTOR_BLS_MAP_FP2_TO_G2 :binary@prove) 
    (* WNON (is_map_fp2_to_g2)))
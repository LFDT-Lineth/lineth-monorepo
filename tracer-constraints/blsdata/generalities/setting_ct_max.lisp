(module blsdata)

(defun (ct_max_first_input)
    (+
      (*  CT_MAX_POINT_EVALUATION DATA_POINT_EVALUATION_FLAG)
       (* CT_MAX_SMALL_POINT      DATA_BLS_G1_ADD_FLAG)
       (* CT_MAX_SMALL_POINT      DATA_BLS_G1_MSM_FLAG)
       (* CT_MAX_LARGE_POINT      DATA_BLS_G2_ADD_FLAG)
       (* CT_MAX_LARGE_POINT      DATA_BLS_G2_MSM_FLAG)
       (* CT_MAX_SMALL_POINT      DATA_BLS_PAIRING_CHECK_FLAG)
       (* CT_MAX_MAP_FP_TO_G1     DATA_BLS_MAP_FP_TO_G1_FLAG)
       (* CT_MAX_MAP_FP2_TO_G2    DATA_BLS_MAP_FP2_TO_G2_FLAG)))

(defun (ct_max_second_input)
    (+
      (*  CT_MAX_SMALL_POINT    DATA_BLS_G1_ADD_FLAG)
       (* CT_MAX_SCALAR         DATA_BLS_G1_MSM_FLAG)
       (* CT_MAX_LARGE_POINT    DATA_BLS_G2_ADD_FLAG)
       (* CT_MAX_SCALAR         DATA_BLS_G2_MSM_FLAG)
       (* CT_MAX_LARGE_POINT    DATA_BLS_PAIRING_CHECK_FLAG)))

(defconstraint set-ct-max ()
    (eq! CT_MAX 
         (+
           (*  (ct_max_first_input)  IS_FIRST_INPUT)
            (* (ct_max_second_input) IS_SECOND_INPUT)
            (* INDEX_MAX             (is_result)))))

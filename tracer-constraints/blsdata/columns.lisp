(module blsdata)

(defcolumns
  (STAMP        :i16)
  (ID           :i32)
  (TOTAL_SIZE   :i24)
  (INDEX        :i16)
  (INDEX_MAX    :i16)
  (LIMB         :i128)
  (PHASE        :i16)
  (SUCCESS_BIT  :binary@prove)

  (CT           :i4)
  (CT_MAX       :i4)

  (DATA_POINT_EVALUATION_FLAG             :binary@prove) 
  (DATA_BLS_G1_ADD_FLAG                   :binary@prove)
  (DATA_BLS_G1_MSM_FLAG                   :binary@prove)
  (DATA_BLS_G2_ADD_FLAG                   :binary@prove)
  (DATA_BLS_G2_MSM_FLAG                   :binary@prove)
  (DATA_BLS_PAIRING_CHECK_FLAG            :binary@prove)
  (DATA_BLS_MAP_FP_TO_G1_FLAG             :binary@prove)
  (DATA_BLS_MAP_FP2_TO_G2_FLAG            :binary@prove)
  
  (RSLT_POINT_EVALUATION_FLAG             :binary@prove)
  (RSLT_BLS_G1_ADD_FLAG                   :binary@prove)
  (RSLT_BLS_G1_MSM_FLAG                   :binary@prove)
  (RSLT_BLS_G2_ADD_FLAG                   :binary@prove)
  (RSLT_BLS_G2_MSM_FLAG                   :binary@prove)
  (RSLT_BLS_PAIRING_CHECK_FLAG            :binary@prove)
  (RSLT_BLS_MAP_FP_TO_G1_FLAG             :binary@prove)
  (RSLT_BLS_MAP_FP2_TO_G2_FLAG            :binary@prove)
 
  (ACC_INPUTS                                    :i16)
  (BYTE_DELTA                                    :byte@prove)

  (MALFORMED_DATA_INTERNAL_BIT                    :binary@prove)
  (MALFORMED_DATA_INTERNAL_ACC                    :binary@prove)
  (MALFORMED_DATA_INTERNAL_ACC_TOT                :binary@prove)
  (MALFORMED_DATA_EXTERNAL_BIT                    :binary@prove)
  (MALFORMED_DATA_EXTERNAL_ACC                    :binary@prove)
  (MALFORMED_DATA_EXTERNAL_ACC_TOT                :binary@prove)
  (WELLFORMED_DATA_TRIVIAL                        :binary@prove)
  (WELLFORMED_DATA_NONTRIVIAL                     :binary@prove)

  (IS_FIRST_INPUT                                :binary@prove)
  (IS_SECOND_INPUT                               :binary@prove)
  (IS_INFINITY                                   :binary@prove)
  (NONTRIVIAL_PAIR_OF_POINTS_BIT                 :binary@prove)
  (NONTRIVIAL_PAIR_OF_POINTS_ACC                 :binary@prove)

  ;; Circuit selector columns are defined using defcomputedcolumn in circuit_selectors.lisp

  (WCP_FLAG     :binary@prove)
  (WCP_ARG1_HI  :i128)
  (WCP_ARG1_LO  :i128)
  (WCP_ARG2_HI  :i128)
  (WCP_ARG2_LO  :i128)
  (WCP_RES      :binary)
  (WCP_INST     :byte :display :opcode)
)

;; aliases
(defalias
  MINT_BIT                      MALFORMED_DATA_INTERNAL_BIT
  MINT_ACC                      MALFORMED_DATA_INTERNAL_ACC
  MINT                          MALFORMED_DATA_INTERNAL_ACC_TOT
  MEXT_BIT                      MALFORMED_DATA_EXTERNAL_BIT
  MEXT_ACC                      MALFORMED_DATA_EXTERNAL_ACC
  MEXT                          MALFORMED_DATA_EXTERNAL_ACC_TOT
  WTRV                          WELLFORMED_DATA_TRIVIAL
  WNON                          WELLFORMED_DATA_NONTRIVIAL
  NONTRIVIAL_POP_BIT            NONTRIVIAL_PAIR_OF_POINTS_BIT
  NONTRIVIAL_POP_ACC            NONTRIVIAL_PAIR_OF_POINTS_ACC
  CS_POINT_EVALUATION           CIRCUIT_SELECTOR_POINT_EVALUATION
  CS_POINT_EVALUATION_FAILURE   CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE
  CS_C1_MEMBERSHIP              CIRCUIT_SELECTOR_C1_MEMBERSHIP
  CS_G1_MEMBERSHIP              CIRCUIT_SELECTOR_G1_MEMBERSHIP
  CS_C2_MEMBERSHIP              CIRCUIT_SELECTOR_C2_MEMBERSHIP
  CS_G2_MEMBERSHIP              CIRCUIT_SELECTOR_G2_MEMBERSHIP
  CS_BLS_PAIRING_CHECK          CIRCUIT_SELECTOR_BLS_PAIRING_CHECK
  CS_BLS_G1_ADD                 CIRCUIT_SELECTOR_BLS_G1_ADD
  CS_BLS_G2_ADD                 CIRCUIT_SELECTOR_BLS_G2_ADD
  CS_BLS_G1_MSM                 CIRCUIT_SELECTOR_BLS_G1_MSM
  CS_BLS_G2_MSM                 CIRCUIT_SELECTOR_BLS_G2_MSM
  CS_BLS_MAP_FP_TO_G1           CIRCUIT_SELECTOR_BLS_MAP_FP_TO_G1
  CS_BLS_MAP_FP2_TO_G2          CIRCUIT_SELECTOR_BLS_MAP_FP2_TO_G2
)



(module hub)

(defperspective scenario

  ;; selector
  PEEK_AT_SCENARIO

  ;; scenario columns
  (
  ;; CALL related scenario columns
   (CALL_EXCEPTION                                   :binary@prove)
   (CALL_ABORT_WILL_REVERT                           :binary@prove)
   (CALL_ABORT_WONT_REVERT                           :binary@prove)
   ;; call to precompiles related
   (CALL_PRC_FAILURE                                 :binary@prove)
   (CALL_PRC_SUCCESS_CALLER_WILL_REVERT              :binary@prove)
   (CALL_PRC_SUCCESS_CALLER_WONT_REVERT              :binary@prove)
   ;; call to smart contract related
   (CALL_SMC_FAILURE_CALLER_WILL_REVERT              :binary@prove)
   (CALL_SMC_FAILURE_CALLER_WONT_REVERT              :binary@prove)
   (CALL_SMC_SUCCESS_CALLER_WILL_REVERT              :binary@prove)
   (CALL_SMC_SUCCESS_CALLER_WONT_REVERT              :binary@prove)
   ;; call to externally owned accounts related
   (CALL_EOA_SUCCESS_CALLER_WILL_REVERT              :binary@prove)
   (CALL_EOA_SUCCESS_CALLER_WONT_REVERT              :binary@prove)

   ;; Create related
   (CREATE_EXCEPTION                                 :binary@prove)
   (CREATE_ABORT                                     :binary@prove)
   (CREATE_FAILURE_CONDITION_WILL_REVERT             :binary@prove)
   (CREATE_FAILURE_CONDITION_WONT_REVERT             :binary@prove)
   (CREATE_EMPTY_INIT_CODE_WILL_REVERT               :binary@prove)
   (CREATE_EMPTY_INIT_CODE_WONT_REVERT               :binary@prove)
   (CREATE_NONEMPTY_INIT_CODE_FAILURE_WILL_REVERT    :binary@prove)
   (CREATE_NONEMPTY_INIT_CODE_FAILURE_WONT_REVERT    :binary@prove)
   (CREATE_NONEMPTY_INIT_CODE_SUCCESS_WILL_REVERT    :binary@prove)
   (CREATE_NONEMPTY_INIT_CODE_SUCCESS_WONT_REVERT    :binary@prove)

   ;; Return related
   (RETURN_EXCEPTION                                 :binary@prove)
   (RETURN_FROM_MESSAGE_CALL_WILL_TOUCH_RAM          :binary@prove)
   (RETURN_FROM_MESSAGE_CALL_WONT_TOUCH_RAM          :binary@prove)
   (RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WILL_REVERT    :binary@prove)
   (RETURN_FROM_DEPLOYMENT_EMPTY_CODE_WONT_REVERT    :binary@prove)
   (RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WILL_REVERT :binary@prove)
   (RETURN_FROM_DEPLOYMENT_NONEMPTY_CODE_WONT_REVERT :binary@prove)

   ;; precompile related
   ;; precompile flags
   (PRC_ECRECOVER                                    :binary@prove)
   (PRC_SHA2-256                                     :binary@prove)
   (PRC_RIPEMD-160                                   :binary@prove)
   (PRC_IDENTITY                                     :binary@prove)
   (PRC_MODEXP                                       :binary@prove)
   (PRC_ECADD                                        :binary@prove)
   (PRC_ECMUL                                        :binary@prove)
   (PRC_ECPAIRING                                    :binary@prove)
   (PRC_BLAKE2f                                      :binary@prove)
   ;; Cancun precompiles
   (PRC_POINT_EVALUATION                             :binary@prove)
   ;; Prague precompiles
   (PRC_BLS_G1_ADD                                   :binary@prove)
   (PRC_BLS_G1_MSM                                   :binary@prove)
   (PRC_BLS_G2_ADD                                   :binary@prove)
   (PRC_BLS_G2_MSM                                   :binary@prove)
   (PRC_BLS_PAIRING_CHECK                            :binary@prove)
   (PRC_BLS_MAP_FP_TO_G1                             :binary@prove)
   (PRC_BLS_MAP_FP2_TO_G2                            :binary@prove)
   ;; Osaka precompiles
   (PRC_P256_VERIFY                                  :binary@prove)
   ;; execution paths
   (PRC_SUCCESS_CALLER_WILL_REVERT                   :binary@prove)
   (PRC_SUCCESS_CALLER_WONT_REVERT                   :binary@prove)
   (PRC_FAILURE_KNOWN_TO_HUB                         :binary@prove)
   (PRC_FAILURE_KNOWN_TO_RAM                         :binary@prove)
   ;; gas parameters (RETURN_GAS is a prediction)
   (PRC_CALLER_GAS                                   :i64)
   (PRC_CALLEE_GAS                                   :i64)
   (PRC_RETURN_GAS                                   :i64)
   ;; duplicate values for precompile calls
   (PRC_CDO                                          :i32)
   (PRC_CDS                                          :i32)
   (PRC_RAO                                          :i32)
   (PRC_RAC                                          :i32)

   ;; SELFDESTRUCT related
   (SELFDESTRUCT_EXCEPTION                           :binary@prove)
   (SELFDESTRUCT_WILL_REVERT                         :binary@prove)
   (SELFDESTRUCT_WONT_REVERT_ALREADY_MARKED          :binary@prove)
   (SELFDESTRUCT_WONT_REVERT_NOT_YET_MARKED          :binary@prove)))

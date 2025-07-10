(module rlptxn)

(defcolumns
  (USER_TXN_NUMBER                  :i24)
  (TXN                              :binary@prove)
  (CMP                              :binary@prove)
  (LIMB_CONSTRUCTED                 :binary@prove)
  (LT                               :binary)
  (LX                               :binary)
  (INDEX_LT                         :i32)
  (INDEX_LX                         :i32)
  (RLP_LT_BYTESIZE                  :i32)
  (RLP_LX_BYTESIZE                  :i32)
  (TO_HASH_BY_PROVER                :binary)
  (CODE_FRAGMENT_INDEX              :i24)
  (TYPE_0                           :binary@prove)
  (TYPE_1                           :binary@prove)
  (TYPE_2                           :binary@prove)
  (TYPE_3                           :binary@prove)
  (TYPE_4                           :binary@prove)
  (IS_RLP_PREFIX                    :binary@prove)
  (IS_CHAIN_ID                      :binary@prove)
  (IS_NONCE                         :binary@prove)
  (IS_GAS_PRICE                     :binary@prove)
  (IS_MAX_PRIORITY_FEE_PER_GAS      :binary@prove)
  (IS_MAX_FEE_PER_GAS               :binary@prove)
  (IS_GAS_LIMIT                     :binary@prove)
  (IS_TO                            :binary@prove)
  (IS_VALUE                         :binary@prove)
  (IS_DATA                          :binary@prove)
  (IS_ACCESS_LIST                   :binary@prove)
  (IS_BETA                          :binary@prove)
  (IS_Y                             :binary@prove)
  (IS_R                             :binary@prove)
  (IS_S                             :binary@prove)
  (PHASE_END                        :binary)
  (CT                               :i8)
  (CT_MAX                           :i8)
  (DONE                             :binary)
  (REPLAY_PROTECTION                :binary)
  (Y_PARITY                         :binary)
)


(defperspective txn
;; selector
TXN
(
  (TX_TYPE                          :i8)
  (IS_DEPLOYMENT                    :binary)
  (CHAIN_ID                         :i64)
  (NONCE                            :i64)
  (GAS_PRICE                        :i64)
  (MAX_PRIORITY_FEE_PER_GAS         :i64)
  (MAX_FEE_PER_GAS                  :i64)
  (GAS_LIMIT                        :i64)
  (TO_HI                            :i32)
  (TO_LO                            :i128)
  (VALUE                            :i96)
  (NUMBER_OF_ZERO_BYTES             :i32)
  (NUMBER_OF_NONZERO_BYTES          :i32)
  (NUMBER_OF_PREWARMED_ADDRESSES    :i32)
  (NUMBER_OF_PREWARMED_STORAGE_KEYS :i32)
  (REQUIRES_EVM_EXECUTION           :binary)
))

(defperspective cmp
;; selector
CMP
(
  (LIMB                             :i128)
  (nBYTES                           :i8)
  (TRM_FLAG                         :binary)
  (RLP_UTILS_FLAG                   :binary)
  (INST                             :i8)
  (EXO_DATA_1                       :i128)
  (EXO_DATA_2                       :i128)
  (EXO_DATA_3                       :binary)
  (EXO_DATA_4                       :binary)
  (EXO_DATA_5                       :binary)
  (EXO_DATA_6                       :i128)
  (EXO_DATA_7                       :i128)
  (EXO_DATA_8                       :i8)
  (IS_PREFIX                        :binary)
  (IS_ADDRESS                       :binary)
  (IS_STORAGE                       :binary)
  (TMP                              :i32 :array [5])
  (TMP6                             :i128)
  (TMP7                             :i32)
))

;; aliases
(defalias
  LC  LIMB_CONSTRUCTED
  CFI CODE_FRAGMENT_INDEX)



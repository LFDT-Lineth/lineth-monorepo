(module rlptxn)

(defcolumns
  ;; ( USER_TXN_NUMBER                  :i24 ) ;; defcomputed column
  ( TXN                           :binary@prove )
  ( CMP                           :binary@prove )
  ( LIMB_CONSTRUCTED              :binary@prove )
  ( LT                            :binary@prove )
  ( LX                            :binary@prove )
  ;; ( INDEX_LT                      :i32 ) ;; defcomputed column
  ;; ( INDEX_LX                      :i32 ) ;; defcomputed column
  ( LT_BYTE_SIZE_COUNTDOWN        :i32          )
  ( LX_BYTE_SIZE_COUNTDOWN        :i32          )
  ;; ( TO_HASH_BY_PROVER             :binary ) ;; defcomputed column
  ( CODE_FRAGMENT_INDEX           :i24          )
  ( TYPE_0                        :binary@prove )
  ( TYPE_1                        :binary@prove )
  ( TYPE_2                        :binary@prove )
  ;; ( TYPE_3                        :binary@prove )
  ( TYPE_4                        :binary@prove )
  ( IS_RLP_PREFIX                 :binary@prove )
  ( IS_CHAIN_ID                   :binary@prove )
  ( IS_NONCE                      :binary@prove )
  ( IS_GAS_PRICE                  :binary@prove )
  ( IS_GAS_LIMIT                  :binary@prove )
  ( IS_TO                         :binary@prove )
  ( IS_VALUE                      :binary@prove )
  ( IS_DATA                       :binary@prove )
  ( IS_ACCESS_LIST                :binary@prove )
  ( IS_AUTHORIZATION_LIST         :binary@prove )
  ( IS_BETA                       :binary@prove )
  ( IS_MAX_PRIORITY_FEE_PER_GAS   :binary@prove )
  ( IS_MAX_FEE_PER_GAS            :binary@prove )
  ( IS_Y                          :binary@prove )
  ( IS_R                          :binary@prove )
  ( IS_S                          :binary@prove )
  ;; ( PHASE_END                     :binary ) ;; defcomputed column
  ( CT                            :i32          ) ;; Linea call data is capped at 120kB < 2**17 limbs = 2**13 bytes
  ( CT_MAX                        :i32          ) ;; i16 is not enough for ref tests
  ;; ( DONE                          :binary ) ;; defcomputed column
  ( REPLAY_PROTECTION             :binary@prove )
  ( Y_PARITY                      :binary@prove )
  ( REQUIRES_EVM_EXECUTION        :binary       )
  ( IS_DEPLOYMENT                 :binary       )
  ( NUMBER_OF_AUTHORIZATIONS      :i10          )
  ( IS_PREFIX_OF_ACCESS_LIST_ITEM :binary@prove )
  ( IS_PREFIX_OF_STORAGE_KEY_LIST :binary@prove )
  ( IS_ACCESS_LIST_ADDRESS        :binary@prove )
  ( IS_ACCESS_LIST_STORAGE_KEY    :binary@prove )
  )

;; aliases
(defalias
  LC  LIMB_CONSTRUCTED
  CFI CODE_FRAGMENT_INDEX
  )

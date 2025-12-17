(module rlptxn)

(defperspective txn
                ;; selector
                TXN
                (
                 ( TX_TYPE                          :i8   )
                 ( CHAIN_ID                         :i64  )
                 ( NONCE                            :i64  )
                 ( GAS_PRICE                        :i64  )
                 ( MAX_PRIORITY_FEE_PER_GAS         :i64  )
                 ( MAX_FEE_PER_GAS                  :i64  )
                 ( GAS_LIMIT                        :i25  )
                 ( TO_HI                            :i32  )
                 ( TO_LO                            :i128 )
                 ( VALUE                            :i96  )
                 ( NUMBER_OF_ZERO_BYTES             :i32  )
                 ( NUMBER_OF_NONZERO_BYTES          :i32  )
                 ( NUMBER_OF_PREWARMED_ADDRESSES    :i24  )
                 ( NUMBER_OF_PREWARMED_STORAGE_KEYS :i24  )
                 ))

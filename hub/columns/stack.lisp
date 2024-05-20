(module hub)

(defperspective stack

  ;; selector
  PEEK_AT_STACK
  (;; stack items
   ( STACK_ITEM_HEIGHT       :array [4])
   ( STACK_ITEM_VALUE_HI     :array [4])
   ( STACK_ITEM_VALUE_LO     :array [4])
   ( STACK_ITEM_POP          :array [4] :binary@prove)
   ( STACK_ITEM_STAMP        :array [4])

   ;; instruction and instruction decoded flags
   (INSTRUCTION              :display :opcode)
   ( STATIC_GAS              :i32    )  ;; TODO: vastly exagerated, shouldn't carry values greater than 32_000 so :i16 should suffice)
   ( ACC_FLAG                :binary )
   ( ADD_FLAG                :binary )
   ( BIN_FLAG                :binary )
   ( BTC_FLAG                :binary )
   ( CALL_FLAG               :binary )
   ( CON_FLAG                :binary )
   ( COPY_FLAG               :binary )
   ( CREATE_FLAG             :binary )
   ( DUP_FLAG                :binary )
   ( EXT_FLAG                :binary )
   ( HALT_FLAG               :binary )
   ( INVALID_FLAG            :binary )
   ( JUMP_FLAG               :binary )
   ( KEC_FLAG                :binary )
   ( LOG_FLAG                :binary )
   ( MACHINE_STATE_FLAG      :binary )
   ( MOD_FLAG                :binary )
   ( MUL_FLAG                :binary )
   ( PUSHPOP_FLAG            :binary )
   ( SHF_FLAG                :binary )
   ( STACKRAM_FLAG           :binary )
   ( STO_FLAG                :binary )
   ( SWAP_FLAG               :binary )
   ( TXN_FLAG                :binary )
   ( WCP_FLAG                :binary )

   ;; auxiliary flags to simplify constraints on exceptions
   ;; likely to disappear
   ( MXP_FLAG                :binary )
   ( STATIC_FLAG             :binary )
   ( DEC_FLAG                :array[4] :binary )

   ;; stack popping / pushing parameters
   ( ALPHA                   :byte )
   ( DELTA                   :byte )
   ( NB_REMOVED              :byte )
   ( NB_ADDED                :byte )

   ;; jump and push related
   ( PUSH_VALUE_HI           :i128 )
   ( PUSH_VALUE_LO           :i128 )
   ( JUMP_DESTINATION_VETTING_REQUIRED :binary@prove )

   ;; exception flags
   ( OPCX                    :binary@prove )
   ( SUX                     :binary@prove )
   ( SOX                     :binary@prove )
   ( MXPX                    :binary@prove )
   ( OOGX                    :binary@prove )
   ( RDCX                    :binary@prove )
   ( JUMPX                   :binary@prove )
   ( STATICX                 :binary@prove )
   ( SSTOREX                 :binary@prove )
   ( ICPX                    :binary@prove )
   ( MAXCSX                  :binary@prove )

   ;; hash info related
   ( HASH_INFO_FLAG          :binary@prove )
   ( HASH_INFO_SIZE          :i32  )
   ( HASH_INFO_KECCAK_HI     :i128 )
   ( HASH_INFO_KECCAK_LO     :i128 )

   ;; log info related
   (LOG_INFO_FLAG :binary@prove)
   )

  (defalias
   INST
   INSTRUCTION
   ))



(module hub_v2)

(defperspective stack

  ;; selector
  PEEK_AT_STACK
  (;; stack items
   (STACK_ITEM_HEIGHT :array [4])
   (STACK_ITEM_VALUE_HI :array [4])
   (STACK_ITEM_VALUE_LO :array [4])
   (STACK_ITEM_POP :array [4] :binary@prove)
   (STACK_ITEM_STAMP :array [4])
   ;; instruction and instruction decoded flags
   (INSTRUCTION :display :opcode)
   STATIC_GAS
   (ACC_FLAG :binary)
   (ADD_FLAG :binary)
   (BIN_FLAG :binary)
   (BTC_FLAG :binary)
   (CALL_FLAG :binary)
   (CON_FLAG :binary)
   (COPY_FLAG :binary)
   (CREATE_FLAG :binary)
   (DUP_FLAG :binary)
   (EXT_FLAG :binary)
   (HALT_FLAG :binary)
   (INVALID_FLAG :binary)
   (JUMP_FLAG :binary)
   (KEC_FLAG :binary)
   (LOG_FLAG :binary)
   (MACHINE_STATE_FLAG :binary)
   (MOD_FLAG :binary)
   (MUL_FLAG :binary)
   (PUSHPOP_FLAG :binary)
   (SHF_FLAG :binary)
   (STACKRAM_FLAG :binary)
   (STO_FLAG :binary)
   (SWAP_FLAG :binary)
   (TXN_FLAG :binary)
   (WCP_FLAG :binary)
   ;; auxiliary flags to simplify constraints on exceptions
   ;; likely to disappear
   (MXP_FLAG :binary)
   (STATIC_FLAG :binary)
   (DEC_FLAG :array [4] :binary)
   ;; stack popping / pushing parameters
   ALPHA
   DELTA
   NB_REMOVED
   NB_ADDED
   ;; jump and push related
   PUSH_VALUE_HI
   PUSH_VALUE_LO
   (JUMP_DESTINATION_VETTING_REQUIRED :binary)
   ;; exception flags
   (OPCX :binary@prove)
   (SUX :binary@prove)
   (SOX :binary@prove)
   (MXPX :binary@prove)
   (OOGX :binary@prove)
   (RDCX :binary@prove)
   (JUMPX :binary@prove)
   (STATICX :binary@prove)
   (SSTOREX :binary@prove)
   (ICPX :binary@prove)
   (MAXCSX :binary@prove)
   ;; hash info related
   (HASH_INFO_FLAG :binary@prove)
   HASH_INFO_SIZE
   HASH_INFO_KEC_HI
   HASH_INFO_KEC_LO
   ;; log info related
   (LOG_INFO_FLAG :binary@prove))
  (defalias
   INST
   INSTRUCTION))



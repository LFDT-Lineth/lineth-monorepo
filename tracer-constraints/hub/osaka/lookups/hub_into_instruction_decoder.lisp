(defun (hub-into-instruction-decoder-trigger) hub.PEEK_AT_STACK)

(defclookup hub-into-instdecoder
  ;; target columns
  (
   instdecoder.OPCODE
   instdecoder.STATIC_GAS
   instdecoder.TWO_LINE_INSTRUCTION
   instdecoder.FLAG_1
   instdecoder.FLAG_2
   instdecoder.FLAG_3
   instdecoder.FLAG_4
   instdecoder.MXP_FLAG
   instdecoder.STATIC_FLAG
   instdecoder.ALPHA
   instdecoder.DELTA
   ;;
   instdecoder.FAMILY_ACCOUNT
   instdecoder.FAMILY_ADD
   instdecoder.FAMILY_BIN
   instdecoder.FAMILY_BATCH
   instdecoder.FAMILY_CALL
   instdecoder.FAMILY_CONTEXT
   instdecoder.FAMILY_COPY
   instdecoder.FAMILY_MCOPY
   instdecoder.FAMILY_CREATE
   instdecoder.FAMILY_DUP
   instdecoder.FAMILY_EXT
   instdecoder.FAMILY_HALT
   instdecoder.FAMILY_INVALID
   instdecoder.FAMILY_JUMP
   instdecoder.FAMILY_KEC
   instdecoder.FAMILY_LOG
   instdecoder.FAMILY_MACHINE_STATE
   instdecoder.FAMILY_MOD
   instdecoder.FAMILY_MUL
   instdecoder.FAMILY_PUSH_POP
   instdecoder.FAMILY_SHF
   instdecoder.FAMILY_STACK_RAM
   instdecoder.FAMILY_STORAGE
   instdecoder.FAMILY_TRANSIENT
   instdecoder.FAMILY_SWAP
   instdecoder.FAMILY_TRANSACTION
   instdecoder.FAMILY_WCP
  )
  ;; source selector
  (hub-into-instruction-decoder-trigger)
  ;; source columns
  (
   hub.stack/INSTRUCTION
   hub.stack/STATIC_GAS
   hub.TWO_LINE_INSTRUCTION
   [hub.stack/DEC_FLAG 1]
   [hub.stack/DEC_FLAG 2]
   [hub.stack/DEC_FLAG 3]
   [hub.stack/DEC_FLAG 4]
   hub.stack/MXP_FLAG
   hub.stack/STATIC_FLAG
   hub.stack/ALPHA
   hub.stack/DELTA
   ;;
   hub.stack/ACC_FLAG
   hub.stack/ADD_FLAG
   hub.stack/BIN_FLAG
   hub.stack/BTC_FLAG
   hub.stack/CALL_FLAG
   hub.stack/CON_FLAG
   hub.stack/COPY_FLAG
   hub.stack/MCOPY_FLAG
   hub.stack/CREATE_FLAG
   hub.stack/DUP_FLAG
   hub.stack/EXT_FLAG
   hub.stack/HALT_FLAG
   hub.stack/INVALID_FLAG
   hub.stack/JUMP_FLAG
   hub.stack/KEC_FLAG
   hub.stack/LOG_FLAG
   hub.stack/MACHINE_STATE_FLAG
   hub.stack/MOD_FLAG
   hub.stack/MUL_FLAG
   hub.stack/PUSHPOP_FLAG
   hub.stack/SHF_FLAG
   hub.stack/STACKRAM_FLAG
   hub.stack/STO_FLAG
   hub.stack/TRANS_FLAG
   hub.stack/SWAP_FLAG
   hub.stack/TXN_FLAG
   hub.stack/WCP_FLAG
   )
)

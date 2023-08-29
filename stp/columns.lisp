(module stp)

(defcolumns
  STAMP
  CT
  CT_MAX
  (INSTRUCTION_TYPE                    :BOOLEAN)
  (CALL_CAN_TRANSFER_VALUE             :BOOLEAN)
  ;;
  GAS_ACTUAL
  GAS_MXP
  GAS_COST
  GAS_STIPEND
  ;;
  GAS_HI
  GAS_LO
  VAL_HI
  VAL_LO
  ;;
  (TO_EXISTS                           :BOOLEAN)
  (TO_WARM                             :BOOLEAN)
  (OUT_OF_GAS_EXCEPTION                :BOOLEAN)
  ;;
  (ABORT                               :BOOLEAN)
  CALL_STACK_DEPTH
  FROM_BALANCE
  TO_NONCE
  (TO_HAS_CODE                         :BOOLEAN)
  ;;
  (WCP_FLAG                            :BOOLEAN)
  (MOD_FLAG                            :BOOLEAN)
  (EXOGENOUS_MODULE_INSTRUCTION        :BYTE) 
  ARG_1_HI
  ARG_1_LO
  ARG_2_LO
  RES_LO
  ;; TODO find solution for this hack
  ZERO
  )

(defalias
  INST_TYPE         INSTRUCTION_TYPE
  CCTV              CALL_CAN_TRANSFER_VALUE
  CSD               CALL_STACK_DEPTH
  OOGX              OUT_OF_GAS_EXCEPTION
  EXO_INST          EXOGENOUS_MODULE_INSTRUCTION
  )

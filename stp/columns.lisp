(module stp)

(defcolumns
  STAMP
  CT
  CT_MAX
  (INSTRUCTION_TYPE                    :binary)
  (CALL_CAN_TRANSFER_VALUE             :binary)
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
  (TO_EXISTS                           :binary)
  (TO_WARM                             :binary)
  (OUT_OF_GAS_EXCEPTION                :binary)
  ;;
  (ABORT                               :binary)
  CALL_STACK_DEPTH
  FROM_BALANCE
  TO_NONCE
  (TO_HAS_CODE                         :binary)
  ;;
  (WCP_FLAG                            :binary)
  (MOD_FLAG                            :binary)
  (EXOGENOUS_MODULE_INSTRUCTION        :byte) 
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

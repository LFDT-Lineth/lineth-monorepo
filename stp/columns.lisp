(module stp)

(defcolumns 
  STAMP
  CT
  CT_MAX
  (INSTRUCTION :byte :display :opcode)
  (IS_CREATE :binary)
  (IS_CREATE2 :binary)
  (IS_CALL :binary)
  (IS_CALLCODE :binary)
  (IS_DELEGATECALL :binary)
  (IS_STATICCALL :binary)
  ;;
  GAS_HI
  GAS_LO
  VAL_HI
  VAL_LO
  ;;
  (EXISTS :binary)
  (WARM :binary)
  (OUT_OF_GAS_EXCEPTION :binary)
  ;;
  GAS_ACTUAL
  GAS_MXP
  GAS_UPFRONT
  GAS_OOPKT
  GAS_STIPEND
  ;;
  (WCP_FLAG :binary)
  (MOD_FLAG :binary)
  (EXOGENOUS_MODULE_INSTRUCTION :byte :display :opcode)
  ARG_1_HI
  ARG_1_LO
  ARG_2_LO
  RES_LO)

(defalias 
  OOGX     OUT_OF_GAS_EXCEPTION
  EXO_INST EXOGENOUS_MODULE_INSTRUCTION)



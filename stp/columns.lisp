(module stp)

(defcolumns 
  (STAMP :i24)
  (CT :byte)
  (CT_MAX :byte)
  (INSTRUCTION :byte :display :opcode)
  (IS_CREATE :binary@prove)
  (IS_CREATE2 :binary@prove)
  (IS_CALL :binary@prove)
  (IS_CALLCODE :binary@prove)
  (IS_DELEGATECALL :binary@prove)
  (IS_STATICCALL :binary@prove)
  ;;
  (GAS_HI :i128)
  (GAS_LO :i128)
  (VAL_HI :i128)
  (VAL_LO :i128)
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
  (ARG_1_HI :i128)
  (ARG_1_LO :i128)
  (ARG_2_LO :i128)
  (RES_LO :i128))

(defalias 
  OOGX     OUT_OF_GAS_EXCEPTION
  EXO_INST EXOGENOUS_MODULE_INSTRUCTION)



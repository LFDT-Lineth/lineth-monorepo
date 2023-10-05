(module rom)

(defcolumns 
  ;;;; these define the loading scenarios
  ;;(IS_INITCODE :BOOLEAN)
  ;;
  LIMB
  ADDRESS_HI
  ADDRESS_LO
  ;;
  ;;;; beating heart of a code fragment
  COUNTER
  ;;(CYCLIC_BIT :BOOLEAN)
  ;;
  ;;;; byte code "metadata"
  CODESIZE
  ;;CODEHASH_HI
  ;;CODEHASH_LO
  ;;CURRENT_CODEWORD
  ;;
  ;;;; related to constructing push values
  ;;(IS_PUSH			:BOOLEAN)
  ;;(IS_PUSH_DATA		:BOOLEAN)
  ;;PUSH_PARAMETER
  ;;PUSH_PARAMETER_OFFSET
  ;;PUSH_VALUE_HI
  ;;PUSH_VALUE_LO
  ;;PUSH_VALUE_ACC_HI
  ;;PUSH_VALUE_ACC_LO
  ;;(PUSH_FUNNEL_BIT	:BOOLEAN)
  ;;
  ;;;; related to bytecode itself and padding
  (PADDED_BYTECODE_BYTE :byte)
  ;;(OPCODE					:BYTE)
  ;;(PADDING_BIT		:BOOLEAN)
  ;;PC
  (CODESIZE_REACHED :boolean)
  ;;(IS_BYTECODE		:BOOLEAN)
  CODE_FRAGMENT_INDEX
  PROGRAMME_COUNTER)

(defalias 
  PC  PROGRAMME_COUNTER
  CFI CODE_FRAGMENT_INDEX
  CT  COUNTER)



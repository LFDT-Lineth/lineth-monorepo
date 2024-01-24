(module rom)

(defcolumns 
  (CODE_FRAGMENT_INDEX :i32)
  (CODE_FRAGMENT_INDEX_INFTY :i32)
  (CODE_SIZE :i32)
  (CODESIZE_REACHED :binary@prove)
  (PROGRAMME_COUNTER :i32)
  (LIMB :i128)
  (nBYTES :byte)
  (nBYTES_ACC :byte)
  (INDEX :i32)
  (COUNTER :byte)
  (COUNTER_MAX :byte)
  (PADDED_BYTECODE_BYTE :byte)
  (ACC :i128)
  (IS_PUSH :binary)
  (PUSH_PARAMETER :byte)
  (COUNTER_PUSH :byte)
  (IS_PUSH_DATA :binary@prove)
  (PUSH_VALUE_HIGH :i128)
  (PUSH_VALUE_LOW  :i128)
  (PUSH_VALUE_ACC  :i128)
  (PUSH_FUNNEL_BIT :binary@prove)
  (OPCODE :byte)
  (VALID_JUMP_DESTINATION :binary))

(defalias 
  PC   PROGRAMME_COUNTER
  CFI  CODE_FRAGMENT_INDEX
  CT   COUNTER
  PBCB PADDED_BYTECODE_BYTE)



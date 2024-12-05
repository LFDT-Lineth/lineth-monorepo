(module wcp)

(defcolumns
  (WORD_COMPARISON_STAMP :i32)
  (COUNTER :byte)
  (CT_MAX :byte)
  (INST :byte :display :opcode)
  (ARGUMENT_1_HI :i128)
  (ARGUMENT_1_LO :i128)
  (ARGUMENT_2_HI :i128)
  (ARGUMENT_2_LO :i128)
  (RESULT :binary@prove)
  (IS_LT :binary@prove)
  (IS_GT :binary@prove)
  (IS_SLT :binary@prove)
  (IS_SGT :binary@prove)
  (IS_EQ :binary@prove)
  (IS_ISZERO :binary@prove)
  (IS_GEQ :binary@prove)
  (IS_LEQ :binary@prove)
  (ONE_LINE_INSTRUCTION :binary)
  (VARIABLE_LENGTH_INSTRUCTION :binary)
  (BITS :binary@prove)
  (NEG_1 :binary@prove)
  (NEG_2 :binary@prove)
  (BYTE_1 :byte@prove)
  (BYTE_2 :byte@prove)
  (BYTE_3 :byte@prove)
  (BYTE_4 :byte@prove)
  (BYTE_5 :byte@prove)
  (BYTE_6 :byte@prove)
  (ACC_1 :i128)
  (ACC_2 :i128)
  (ACC_3 :i128)
  (ACC_4 :i128)
  (ACC_5 :i128)
  (ACC_6 :i128)
  (BIT_1 :binary@prove)
  (BIT_2 :binary@prove)
  (BIT_3 :binary@prove)
  (BIT_4 :binary@prove))

;; aliases
(defalias
  STAMP    WORD_COMPARISON_STAMP
  OLI      ONE_LINE_INSTRUCTION
  VLI      VARIABLE_LENGTH_INSTRUCTION
  CT       COUNTER
  ARG_1_HI ARGUMENT_1_HI
  ARG_1_LO ARGUMENT_1_LO
  ARG_2_HI ARGUMENT_2_HI
  ARG_2_LO ARGUMENT_2_LO
  RES      RESULT)



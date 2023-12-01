(module wcp)

(defcolumns 
  WORD_COMPARISON_STAMP
  (ONE_LINE_INSTRUCTION :binary)
  COUNTER
  (INST :display :opcode)
  ARGUMENT_1_HI
  ARGUMENT_1_LO
  ARGUMENT_2_HI
  ARGUMENT_2_LO
  RESULT_HI
  RESULT_LO
  (BITS :binary)
  (NEG_1 :binary)
  (NEG_2 :binary)
  (BYTE_1 :byte)
  (BYTE_2 :byte)
  (BYTE_3 :byte)
  (BYTE_4 :byte)
  (BYTE_5 :byte)
  (BYTE_6 :byte)
  ACC_1
  ACC_2
  ACC_3
  ACC_4
  ACC_5
  ACC_6
  (BIT_1 :binary)
  (BIT_2 :binary)
  (BIT_3 :binary)
  (BIT_4 :binary))

;; aliases
(defalias 
  STAMP    WORD_COMPARISON_STAMP
  OLI      ONE_LINE_INSTRUCTION
  CT       COUNTER
  ARG_1_HI ARGUMENT_1_HI
  ARG_1_LO ARGUMENT_1_LO
  ARG_2_HI ARGUMENT_2_HI
  ARG_2_LO ARGUMENT_2_LO
  RES_HI   RESULT_HI
  RES_LO   RESULT_LO)



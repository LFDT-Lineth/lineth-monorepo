(module bin)

(defcolumns 
  STAMP
  (ONE_LINE_INSTRUCTION :binary)
  (MLI :binary)
  (COUNTER :byte)
  (INST :byte :display :opcode)
  ARGUMENT_1_HI
  ARGUMENT_1_LO
  ARGUMENT_2_HI
  ARGUMENT_2_LO
  RESULT_HI
  RESULT_LO
  (IS_AND :binary)
  (IS_OR :binary)
  (IS_XOR :binary)
  (IS_NOT :binary)
  (IS_BYTE :binary)
  (IS_SIGNEXTEND :binary)
  (SMALL :binary)
  (BITS :binary)
  (BIT_B_4 :binary)
  (LOW_4 :byte)
  (NEG :binary)
  (BIT_1 :binary)
  (PIVOT :byte)
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
  ;; decoded bytes:
  (XXX_BYTE_HI :byte)
  (XXX_BYTE_LO :byte))

;; aliases
(defalias 
  OLI      ONE_LINE_INSTRUCTION
  CT       COUNTER
  ARG_1_HI ARGUMENT_1_HI
  ARG_1_LO ARGUMENT_1_LO
  ARG_2_HI ARGUMENT_2_HI
  ARG_2_LO ARGUMENT_2_LO
  RES_HI   RESULT_HI
  RES_LO   RESULT_LO)



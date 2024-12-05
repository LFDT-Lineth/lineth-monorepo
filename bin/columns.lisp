(module bin)

(defcolumns
  (STAMP :i32)
  (CT_MAX :byte)
  (COUNTER :byte)
  (INST :byte :display :opcode)
  (ARGUMENT_1_HI :i128)
  (ARGUMENT_1_LO :i128)
  (ARGUMENT_2_HI :i128)
  (ARGUMENT_2_LO :i128)
  (RESULT_HI :i128)
  (RESULT_LO :i128)
  (IS_AND :binary@prove)
  (IS_OR :binary@prove)
  (IS_XOR :binary@prove)
  (IS_NOT :binary@prove)
  (IS_BYTE :binary@prove)
  (IS_SIGNEXTEND :binary@prove)
  (SMALL :binary@prove)
  (BITS :binary@prove)
  (BIT_B_4 :binary@prove)
  (LOW_4 :byte)
  (NEG :binary@prove)
  (BIT_1 :binary@prove)
  (PIVOT :byte)
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
  ;; decoded bytes:
  (XXX_BYTE_HI :byte)
  (XXX_BYTE_LO :byte))

;; aliases
(defalias
  CT       COUNTER
  ARG_1_HI ARGUMENT_1_HI
  ARG_1_LO ARGUMENT_1_LO
  ARG_2_HI ARGUMENT_2_HI
  ARG_2_LO ARGUMENT_2_LO
  RES_HI   RESULT_HI
  RES_LO   RESULT_LO)



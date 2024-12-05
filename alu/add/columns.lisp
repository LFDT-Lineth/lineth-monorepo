(module add)

(defcolumns
  (STAMP :i32)
  (CT_MAX :byte)
  (CT :byte)
  (INST :byte :display :opcode)
  (ARG_1_HI :i128)
  (ARG_1_LO :i128)
  (ARG_2_HI :i128)
  (ARG_2_LO :i128)
  (RES_HI :i128)
  (RES_LO :i128)
  (BYTE_1 :byte@prove)
  (BYTE_2 :byte@prove)
  (ACC_1 :i128)
  (ACC_2 :i128)
  (OVERFLOW :binary@prove))



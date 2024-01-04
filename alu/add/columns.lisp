(module add)

(defcolumns 
  STAMP
  (CT_MAX :byte)
  (CT :byte)
  (INST :byte :display :opcode)
  ARG_1_HI
  ARG_1_LO
  ARG_2_HI
  ARG_2_LO
  RES_HI
  RES_LO
  (BYTE_1 :byte)
  (BYTE_2 :byte)
  ACC_1
  ACC_2
  (OVERFLOW :binary))



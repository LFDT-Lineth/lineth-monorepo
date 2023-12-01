(module mod)

(defcolumns 
  STAMP
  (OLI :binary)
  CT
  (INST :display :opcode)
  (DEC_SIGNED :binary) ;while instruction decoded this provides better If-zero etc stuff 
  (DEC_OUTPUT :binary) ;
  ;
  ARG_1_HI
  ARG_1_LO
  ARG_2_HI
  ARG_2_LO
  RES_HI
  RES_LO
  ;
  (BYTE_1_3 :byte)
  ACC_1_3
  (BYTE_1_2 :byte)
  ACC_1_2
  (BYTE_2_3 :byte)
  ACC_2_3
  (BYTE_2_2 :byte)
  ACC_2_2
  ;
  (BYTE_B_3 :byte)
  ACC_B_3
  (BYTE_B_2 :byte)
  ACC_B_2
  (BYTE_B_1 :byte)
  ACC_B_1
  (BYTE_B_0 :byte)
  ACC_B_0
  ;
  (BYTE_Q_3 :byte)
  ACC_Q_3
  (BYTE_Q_2 :byte)
  ACC_Q_2
  (BYTE_Q_1 :byte)
  ACC_Q_1
  (BYTE_Q_0 :byte)
  ACC_Q_0
  ;
  (BYTE_R_3 :byte)
  ACC_R_3
  (BYTE_R_2 :byte)
  ACC_R_2
  (BYTE_R_1 :byte)
  ACC_R_1
  (BYTE_R_0 :byte)
  ACC_R_0
  ;
  (BYTE_DELTA_3 :byte)
  ACC_DELTA_3
  (BYTE_DELTA_2 :byte)
  ACC_DELTA_2
  (BYTE_DELTA_1 :byte)
  ACC_DELTA_1
  (BYTE_DELTA_0 :byte)
  ACC_DELTA_0
  ;
  (BYTE_H_2 :byte)
  ACC_H_2
  (BYTE_H_1 :byte)
  ACC_H_1
  (BYTE_H_0 :byte)
  ACC_H_0
  ;
  (CMP_1 :binary)
  (CMP_2 :binary)
  ;
  (MSB_1 :binary)
  (MSB_2 :binary))



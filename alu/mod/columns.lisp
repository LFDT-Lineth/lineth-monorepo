(module mod)

(defcolumns
    STAMP
    (OLI :BOOLEAN)
    CT
    INST
    (DEC_SIGNED :BOOLEAN) ;while instruction decoded this provides better If-zero etc stuff 
    (DEC_OUTPUT :BOOLEAN) ;
    ;
    ARG_1_HI
    ARG_1_LO
    ARG_2_HI
    ARG_2_LO
    RES_HI
    RES_LO
    ;
    (BYTE_1_3 :BYTE)        ACC_1_3
    (BYTE_1_2 :BYTE)        ACC_1_2
    (BYTE_2_3 :BYTE)        ACC_2_3
    (BYTE_2_2 :BYTE)        ACC_2_2
    ;
    (BYTE_B_3 :BYTE)        ACC_B_3
    (BYTE_B_2 :BYTE)        ACC_B_2
    (BYTE_B_1 :BYTE)        ACC_B_1
    (BYTE_B_0 :BYTE)        ACC_B_0
    ;
    (BYTE_Q_3 :BYTE)        ACC_Q_3
    (BYTE_Q_2 :BYTE)        ACC_Q_2
    (BYTE_Q_1 :BYTE)        ACC_Q_1
    (BYTE_Q_0 :BYTE)        ACC_Q_0
    ;
    (BYTE_R_3 :BYTE)        ACC_R_3
    (BYTE_R_2 :BYTE)        ACC_R_2
    (BYTE_R_1 :BYTE)        ACC_R_1
    (BYTE_R_0 :BYTE)        ACC_R_0
    ;
    (BYTE_DELTA_3 :BYTE)    ACC_DELTA_3
    (BYTE_DELTA_2 :BYTE)    ACC_DELTA_2
    (BYTE_DELTA_1 :BYTE)    ACC_DELTA_1
    (BYTE_DELTA_0 :BYTE)    ACC_DELTA_0
    ;
    (BYTE_H_2 :BYTE)        ACC_H_2
    (BYTE_H_1 :BYTE)        ACC_H_1
    (BYTE_H_0 :BYTE)        ACC_H_0
    ;
    (CMP_1 :BOOLEAN)
    (CMP_2 :BOOLEAN)
    ;
    (MSB_1 :BOOLEAN)
    (MSB_2 :BOOLEAN)
)
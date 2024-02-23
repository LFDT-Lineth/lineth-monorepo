(module ext)

(defcolumns
    STAMP
    (OLI :binary)
    CT
    INST
    ;
    ARG_1_HI
    ARG_1_LO
    ARG_2_HI
    ARG_2_LO
    ARG_3_HI
    ARG_3_LO
    RES_HI
    RES_LO
    ;
    (CMP    :binary@prove)
    (OF_I   :binary@prove)
    (OF_J   :binary@prove)
    (OF_H   :binary@prove)
    (OF_RES :binary@prove) 
    ;
    (BIT_1 :binary@prove)
    (BIT_2 :binary@prove)
    (BIT_3 :binary@prove)
    ;
    (BYTE_A_3 :byte@prove)        ACC_A_3
    (BYTE_A_2 :byte@prove)        ACC_A_2
    (BYTE_A_1 :byte@prove)        ACC_A_1
    (BYTE_A_0 :byte@prove)        ACC_A_0
    ;
    (BYTE_B_3 :byte@prove)        ACC_B_3
    (BYTE_B_2 :byte@prove)        ACC_B_2
    (BYTE_B_1 :byte@prove)        ACC_B_1
    (BYTE_B_0 :byte@prove)        ACC_B_0
    ;
    (BYTE_C_3 :byte@prove)        ACC_C_3
    (BYTE_C_2 :byte@prove)        ACC_C_2
    (BYTE_C_1 :byte@prove)        ACC_C_1
    (BYTE_C_0 :byte@prove)        ACC_C_0
    ;
    (BYTE_Q_7 :byte@prove)        ACC_Q_7
    (BYTE_Q_6 :byte@prove)        ACC_Q_6
    (BYTE_Q_5 :byte@prove)        ACC_Q_5
    (BYTE_Q_4 :byte@prove)        ACC_Q_4
    (BYTE_Q_3 :byte@prove)        ACC_Q_3
    (BYTE_Q_2 :byte@prove)        ACC_Q_2
    (BYTE_Q_1 :byte@prove)        ACC_Q_1
    (BYTE_Q_0 :byte@prove)        ACC_Q_0
    ;
    (BYTE_R_3 :byte@prove)        ACC_R_3
    (BYTE_R_2 :byte@prove)        ACC_R_2
    (BYTE_R_1 :byte@prove)        ACC_R_1
    (BYTE_R_0 :byte@prove)        ACC_R_0
    ;
    (BYTE_DELTA_3 :byte@prove)    ACC_DELTA_3
    (BYTE_DELTA_2 :byte@prove)    ACC_DELTA_2
    (BYTE_DELTA_1 :byte@prove)    ACC_DELTA_1
    (BYTE_DELTA_0 :byte@prove)    ACC_DELTA_0
    ;
    (BYTE_H_5 :byte@prove)        ACC_H_5
    (BYTE_H_4 :byte@prove)        ACC_H_4
    (BYTE_H_3 :byte@prove)        ACC_H_3
    (BYTE_H_2 :byte@prove)        ACC_H_2
    (BYTE_H_1 :byte@prove)        ACC_H_1
    (BYTE_H_0 :byte@prove)        ACC_H_0
    ;
    (BYTE_I_6 :byte@prove)        ACC_I_6
    (BYTE_I_5 :byte@prove)        ACC_I_5
    (BYTE_I_4 :byte@prove)        ACC_I_4
    (BYTE_I_3 :byte@prove)        ACC_I_3
    (BYTE_I_2 :byte@prove)        ACC_I_2
    (BYTE_I_1 :byte@prove)        ACC_I_1
    (BYTE_I_0 :byte@prove)        ACC_I_0
    ;
    (BYTE_J_7 :byte@prove)        ACC_J_7
    (BYTE_J_6 :byte@prove)        ACC_J_6
    (BYTE_J_5 :byte@prove)        ACC_J_5
    (BYTE_J_4 :byte@prove)        ACC_J_4
    (BYTE_J_3 :byte@prove)        ACC_J_3
    (BYTE_J_2 :byte@prove)        ACC_J_2
    (BYTE_J_1 :byte@prove)        ACC_J_1
    (BYTE_J_0 :byte@prove)        ACC_J_0
)

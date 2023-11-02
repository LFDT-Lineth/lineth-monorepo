(module mul)

(defcolumns
    MUL_STAMP
    COUNTER
    (OLI :binary)
    (TINY_BASE          :binary)
    (TINY_EXPONENT      :binary)
    (RESULT_VANISHES    :binary)
    INSTRUCTION
    ARG_1_HI
    ARG_1_LO
    ARG_2_HI
    ARG_2_LO
    RES_HI
    RES_LO
    (BITS :binary)
    ;==========================
    (BYTE_A_3 :byte)    ACC_A_3
    (BYTE_A_2 :byte)    ACC_A_2
    (BYTE_A_1 :byte)    ACC_A_1
    (BYTE_A_0 :byte)    ACC_A_0
    ;==========================
    (BYTE_B_3 :byte)    ACC_B_3
    (BYTE_B_2 :byte)    ACC_B_2
    (BYTE_B_1 :byte)    ACC_B_1
    (BYTE_B_0 :byte)    ACC_B_0
    ;==========================
    (BYTE_C_3 :byte)    ACC_C_3
    (BYTE_C_2 :byte)    ACC_C_2
    (BYTE_C_1 :byte)    ACC_C_1
    (BYTE_C_0 :byte)    ACC_C_0
    ;==========================
    (BYTE_H_3 :byte)    ACC_H_3
    (BYTE_H_2 :byte)    ACC_H_2
    (BYTE_H_1 :byte)    ACC_H_1
    (BYTE_H_0 :byte)    ACC_H_0
    ;==========================
    (EXPONENT_BIT               :binary)
    EXPONENT_BIT_ACCUMULATOR
    (EXPONENT_BIT_SOURCE        :binary)
    (SQUARE_AND_MULTIPLY        :binary)
    BIT_NUM
)

(defalias

    STAMP       MUL_STAMP
    CT          COUNTER
    INST        INSTRUCTION
    EBIT        EXPONENT_BIT
    EACC        EXPONENT_BIT_ACCUMULATOR
    ESRC        EXPONENT_BIT_SOURCE
    SNM         SQUARE_AND_MULTIPLY
    TINYB       TINY_BASE
    TINYE       TINY_EXPONENT
    RESV        RESULT_VANISHES)

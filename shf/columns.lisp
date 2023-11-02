(module shf)

(defcolumns
    SHIFT_STAMP
    COUNTER
    (ONE_LINE_INSTRUCTION :binary)
    ARG_1_HI
    ARG_1_LO
    ARG_2_HI
    ARG_2_LO
    RES_HI
    RES_LO
    INST
    (SHIFT_DIRECTION :binary)
    (BITS :binary)
    (NEG :binary)
    (KNOWN :binary)
    LEFT_ALIGNED_SUFFIX_HIGH        ;decoded
    RIGHT_ALIGNED_PREFIX_HIGH       ;decoded
    LEFT_ALIGNED_SUFFIX_LOW         ;decoded
    RIGHT_ALIGNED_PREFIX_LOW        ;decoded
    ONES                            ;decoded
    LOW_3
    MICRO_SHIFT_PARAMETER
    (BIT_1 :binary)
    (BIT_2 :binary)
    (BIT_3 :binary)
    (BIT_4 :binary)
    (BIT_B_3 :binary)
    (BIT_B_4 :binary)
    (BIT_B_5 :binary)
    (BIT_B_6 :binary)
    (BIT_B_7 :binary)
    (BYTE_1 :byte)
    (BYTE_2 :byte)
    (BYTE_3 :byte)
    (BYTE_4 :byte)
    (BYTE_5 :byte)
    ACC_1
    ACC_2
    ACC_3
    ACC_4
    ACC_5
    SHB_3_HI
    SHB_3_LO
    SHB_4_HI
    SHB_4_LO
    SHB_5_HI
    SHB_5_LO
    SHB_6_HI
    SHB_6_LO
    SHB_7_HI
    SHB_7_LO
    (IS_DATA :binary)     ;turns on exactly when stamp is non zero
    )


;; aliases
(defalias
    STAMP       SHIFT_STAMP
    SHD         SHIFT_DIRECTION
    OLI         ONE_LINE_INSTRUCTION
    µSHP        MICRO_SHIFT_PARAMETER
    CT          COUNTER
    LA_HI       LEFT_ALIGNED_SUFFIX_HIGH
    RA_HI       RIGHT_ALIGNED_PREFIX_HIGH
    LA_LO       LEFT_ALIGNED_SUFFIX_LOW
    RA_LO       RIGHT_ALIGNED_PREFIX_LOW)
(module shf)

(defcolumns
    SHIFT_STAMP
    COUNTER
    (ONE_LINE_INSTRUCTION :BOOLEAN)
    ARG_1_HI
    ARG_1_LO
    ARG_2_HI
    ARG_2_LO
    RES_HI
    RES_LO
    INST
    (SHIFT_DIRECTION :BOOLEAN)
    (BITS :BOOLEAN)
    (NEG :BOOLEAN)
    (KNOWN :BOOLEAN)
    LEFT_ALIGNED_SUFFIX_HIGH        ;decoded
    RIGHT_ALIGNED_PREFIX_HIGH       ;decoded
    LEFT_ALIGNED_SUFFIX_LOW         ;decoded
    RIGHT_ALIGNED_PREFIX_LOW        ;decoded
    ONES                            ;decoded
    LOW_3
    MICRO_SHIFT_PARAMETER
    (BIT_1 :BOOLEAN)
    (BIT_2 :BOOLEAN)
    (BIT_3 :BOOLEAN)
    (BIT_4 :BOOLEAN)
    (BIT_B_3 :BOOLEAN)
    (BIT_B_4 :BOOLEAN)
    (BIT_B_5 :BOOLEAN)
    (BIT_B_6 :BOOLEAN)
    (BIT_B_7 :BOOLEAN)
    (BYTE_1 :BYTE)
    (BYTE_2 :BYTE)
    (BYTE_3 :BYTE)
    (BYTE_4 :BYTE)
    (BYTE_5 :BYTE)
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
    (IS_DATA :BOOLEAN)     ;turns on exactly when stamp is non zero
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
(deflookup lookup-mxp-into-instruction-decoder
    ;; source columns
    (
        instruction-decoder.MXP_TYPE_1
        instruction-decoder.MXP_TYPE_2
        instruction-decoder.MXP_TYPE_3
        instruction-decoder.MXP_TYPE_4
        instruction-decoder.MXP_TYPE_5
        instruction-decoder.BILLING_PER_WORD
        instruction-decoder.BILLING_PER_BYTE
        instruction-decoder.OPCODE
    )
    ;target columns
    (
        [mxp.MXP_TYPE 1]
        [mxp.MXP_TYPE 2]
        [mxp.MXP_TYPE 3]
        [mxp.MXP_TYPE 4]
        [mxp.MXP_TYPE 5]
        mxp.GWORD
        mxp.GBYTE
        mxp.INST
    )
)

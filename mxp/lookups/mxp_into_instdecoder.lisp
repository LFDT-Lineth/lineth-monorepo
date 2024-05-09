(deflookup lookup-mxp-into-instdecoder
    ;; source columns
    (
        instdecoder.MXP_TYPE_1
        instdecoder.MXP_TYPE_2
        instdecoder.MXP_TYPE_3
        instdecoder.MXP_TYPE_4
        instdecoder.MXP_TYPE_5
        instdecoder.BILLING_PER_WORD
        instdecoder.BILLING_PER_BYTE
        instdecoder.OPCODE
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

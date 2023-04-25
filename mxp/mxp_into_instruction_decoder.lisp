(defplookup plookup-mxp-into-instruction-decoder
    ;source columns
    (
        [mxp.MXP_TYPE 1]
        [mxp.MXP_TYPE 2]
        [mxp.MXP_TYPE 3]
        [mxp.MXP_TYPE 4]
        [mxp.MXP_TYPE 5]
        mxp.MXP_GWORD
        mxp.MXP_GBYTE
        mxp.MXP_INST
    )
    ;target columns
    (
        instruction-decoder.MXP_TYPE_1
        instruction-decoder.MXP_TYPE_2
        instruction-decoder.MXP_TYPE_3
        instruction-decoder.MXP_TYPE_4
        instruction-decoder.MXP_TYPE_5
        instruction-decoder.MXP_GWORD
        instruction-decoder.MXP_GBYTE
        instruction-decoder.MXP_INST
    )
)

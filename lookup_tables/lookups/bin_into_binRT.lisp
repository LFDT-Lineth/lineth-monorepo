(deflookup bin-lookup-table-high
    ;reference columns
    (
        binRT.BYTE_ARG_1
        binRT.BYTE_ARG_2
        binRT.AND_BYTE
        binRT.OR_BYTE
        binRT.XOR_BYTE
        binRT.NOT_BYTE
        binRT.IS_IN_RT
    )
    ;source columns 
    (
        bin.BYTE_1
        bin.BYTE_3
        bin.AND_BYTE_HI
        bin.OR_BYTE_HI
        bin.XOR_BYTE_HI
        bin.NOT_BYTE_HI
        bin.IS_DATA
    )
)

(deflookup bin-lookup-table-low
    ;reference columns
    (
        binRT.BYTE_ARG_1
        binRT.BYTE_ARG_2
        binRT.AND_BYTE
        binRT.OR_BYTE
        binRT.XOR_BYTE
        binRT.NOT_BYTE
        binRT.IS_IN_RT
    )
    ;source columns 
    (
        bin.BYTE_2
        bin.BYTE_4
        bin.AND_BYTE_LO
        bin.OR_BYTE_LO
        bin.XOR_BYTE_LO
        bin.NOT_BYTE_LO
        bin.IS_DATA
    )
)
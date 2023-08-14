(defplookup stp-into-wcp
    ; target colums (in WCP)
    (
        wcp.ARG_1_HI
        wcp.ARG_1_LO
        wcp.ARG_2_HI
        wcp.ARG_2_LO
        wcp.RES_HI
        wcp.RES_LO
        wcp.INST
    )
    ; source columns (in STP)
    (
        (* stp.ARG_1_HI     stp.WCP_FLAG)
        (* stp.ARG_1_LO     stp.WCP_FLAG)
        stp.ZERO
        (* stp.ARG_2_LO     stp.WCP_FLAG)
        stp.ZERO
        (* stp.RES_LO       stp.WCP_FLAG)
        (* stp.EXO_INST     stp.WCP_FLAG)
    )
)

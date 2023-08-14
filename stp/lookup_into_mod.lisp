(defplookup stp-into-mod
    ; target columns (in MOD)
    (
        mod.ARG_1_HI
        mod.ARG_1_LO
        mod.ARG_2_HI
        mod.ARG_2_LO
        mod.RES_HI
        mod.RES_LO
        mod.INST
    )
    ; source columns (in STP)
    (
        (* stp.ARG_1_HI   stp.MOD_FLAG)
        (* stp.ARG_1_LO   stp.MOD_FLAG)
        stp.ZERO
        (* stp.ARG_2_LO   stp.MOD_FLAG)
        stp.ZERO
        (* stp.RES_LO       stp.MOD_FLAG)
        (* stp.EXO_INST     stp.MOD_FLAG)
    )
)

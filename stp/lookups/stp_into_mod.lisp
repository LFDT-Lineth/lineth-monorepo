(defclookup
  stp-into-mod
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
  ; source selector
  stp.MOD_FLAG
  ; source columns (in STP)
  (
    stp.ARG_1_HI
    stp.ARG_1_LO
    0
    stp.ARG_2_LO
    0
    stp.RES_LO
    stp.EXO_INST
  ))



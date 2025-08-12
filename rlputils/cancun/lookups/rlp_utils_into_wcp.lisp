(defclookup
  rlp-utils-into-wcp
  ; target columns
  (
    wcp.ARGUMENT_1_HI
    wcp.ARGUMENT_1_LO
    wcp.ARGUMENT_2_HI
    wcp.ARGUMENT_2_LO
    wcp.RESULT
    wcp.CT_MAX
    wcp.INST
    )
  ; source selector
  rlputils.COMPT
  ; source columns
  (
    rlputils.compt/ARG_1_HI
    rlputils.compt/ARG_1_LO
    0
    rlputils.compt/ARG_2_LO
    rlputils.compt/RES
    rlputils.compt/WCP_CT_MAX 
    rlputils.compt/INST
  ))
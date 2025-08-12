(defclookup
  rlp-utils-into-power
  ; target columns
  (
    power.IOMF
    power.EXPONENT
    power.POWER
    )
  ; source selector
  (* rlputils.COMPT rlputils.compt/SHF_FLAG)
  ; source columns
  (
    1
    rlputils.compt/SHF_ARG
    rlputils.compt/SHF_POWER
  ))
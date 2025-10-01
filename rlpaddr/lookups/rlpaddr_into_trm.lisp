(deflookup
  rlpaddr-into-trm
  ;reference columns
  (
    trm.RAW_ADDRESS
    trm.ADDRESS_HI
  )
  ;source columns
  (
    (:: rlpaddr.RAW_ADDR_HI rlpaddr.DEP_ADDR_LO)
    rlpaddr.DEP_ADDR_HI
  ))



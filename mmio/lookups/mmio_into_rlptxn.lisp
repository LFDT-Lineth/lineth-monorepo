(deflookup 
  mmio-into-rlptxn
  ;reference columns
  (
    rlpTxn.ABS_TX_NUM
    rlpTxn.PHASE_ID
    rlpTxn.INDEX_DATA
    rlpTxn.LIMB
    rlpTxn.nBYTES
    rlpTxn.DATA_LO
  )
  ;source columns
  (
    (* mmio.EXO_IS_TXCD mmio.EXO_ID)
    (* mmio.EXO_IS_TXCD mmio.PHASE)
    (* mmio.EXO_IS_TXCD mmio.INDEX_X)
    (* mmio.EXO_IS_TXCD mmio.LIMB)
    (* mmio.EXO_IS_TXCD mmio.SIZE)
    (* mmio.EXO_IS_TXCD mmio.TOTAL_SIZE)
  ))



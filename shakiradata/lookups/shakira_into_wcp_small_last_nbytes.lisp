(deflookup
  shakiradata-into-wcp-small-last-nbytes
  ; target colums (in WCP)
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ; source columns
  (
    0
    (* (is-final-data-row) shakiradata.nBYTES)
    0
    (* (is-final-data-row) LLARGE)
    (* (is-final-data-row) 1)
    (* (is-final-data-row) WCP_INST_LEQ)
  ))



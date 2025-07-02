(defclookup
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
  ; source selector
  (is-final-data-row)
  ; source columns
  (
    0
    shakiradata.nBYTES
    0
    LLARGE
    1
    WCP_INST_LEQ
  ))



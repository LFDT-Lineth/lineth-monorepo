(defclookup
  shakiradata-into-wcp-small-last-nbytes
  ; target colums (in WCP)
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
  )
  ; source selector
  (is-final-data-row)
  ; source columns
  (
    shakiradata.nBYTES
    LLARGE
    1
    WCP_INST_LEQ
  ))



(defun (is-result)
  (force-bin (+ ;; shakiradata.IS_KECCAK_DATA
                shakiradata.IS_KECCAK_RESULT
                ;; shakiradata.IS_SHA2_DATA
                shakiradata.IS_SHA2_RESULT
                ;; shakiradata.IS_RIPEMD_DATA
                shakiradata.IS_RIPEMD_RESULT)))

(defun (is-final-data-row)
  (force-bin (* (is-data) (next (is-result)))))

(defclookup
  shakiradata-into-wcp-nonzero-last-nbytes
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
    0
    1
    EVM_INST_GT
  ))



(defun (is-data)
  (force-bin (+ shakiradata.IS_KECCAK_DATA
                ;; IS_KECCAK_RESULT
                shakiradata.IS_SHA2_DATA
                ;; IS_SHA2_RESULT
                shakiradata.IS_RIPEMD_DATA
                ;; IS_RIPEMD_RESULT
                )))

(defun (is-first-data-row)
  (force-bin (* (is-data)
                (- 1 (prev (is-data))))))

(defclookup
  shakiradata-into-wcp-increasing-id
  ; target colums (in WCP)
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
  )
  ; source selector
  (is-first-data-row)
  ; source columns
  (
    (prev shakiradata.ID)
    shakiradata.ID
    1
    EVM_INST_LT
  ))

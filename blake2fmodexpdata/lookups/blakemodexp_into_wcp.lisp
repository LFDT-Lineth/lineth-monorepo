(defun (blake2fmodexpdata-into-wcp-oob-into-wcp-activation-flag)
  (force-bin (* (~ blake2fmodexpdata.STAMP)
                (- blake2fmodexpdata.STAMP (prev blake2fmodexpdata.STAMP)))))

(defclookup
  blake2fmodexpdata-into-wcp
  ;; target colums (in WCP)
  (
    wcp.ARG_1
    wcp.ARG_2
    wcp.RES
    wcp.INST
  )
  ;; source selector
  (blake2fmodexpdata-into-wcp-oob-into-wcp-activation-flag)
  ;; source columns
  (
    (prev blake2fmodexpdata.ID)
    blake2fmodexpdata.ID
    1
    EVM_INST_LT
  ))



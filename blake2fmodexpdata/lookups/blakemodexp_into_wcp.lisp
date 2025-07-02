(defun (blake2fmodexpdata-into-wcp-oob-into-wcp-activation-flag)
  (force-bin (* (~ blake2fmodexpdata.STAMP)
                (- blake2fmodexpdata.STAMP (prev blake2fmodexpdata.STAMP)))))

(defclookup
  blake2fmodexpdata-into-wcp
  ;; target colums (in WCP)
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ;; source selector
  (blake2fmodexpdata-into-wcp-oob-into-wcp-activation-flag)
  ;; source columns
  (
    0
    (prev blake2fmodexpdata.ID)
    0
    blake2fmodexpdata.ID
    1
    EVM_INST_LT
  ))



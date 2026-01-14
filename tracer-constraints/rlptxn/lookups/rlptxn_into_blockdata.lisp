(defun (sel-rlptxn-to-blockdata) (force-bin (* rlptxn.TXN rlptxn.REPLAY_PROTECTION)))

(defclookup
  rlptxn-into-blockdata
  ;; target columns
  (
    blockdata.IS_CHAINID
    blockdata.DATA_LO
  )
  ;; source selector
  (sel-rlptxn-to-blockdata)
  ;; source columns
  (
    1
    rlptxn.txn/CHAIN_ID
  ))
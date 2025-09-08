(defun   (txn-data-into-wcp-selector)   (* txndata.CMPTN txndata.computation/WCP_FLAG))

(defclookup
  txndata-into-wcp
  ; target columns
  (
   wcp.ARG_1_HI
   wcp.ARG_1_LO
   wcp.ARG_2_HI
   wcp.ARG_2_LO
   wcp.RES
   wcp.INST
   )
  ; source selector
  (txn-data-into-wcp-selector)
  ; source columns
  (
   0
   txndata.computation/ARG_1_LO
   0
   txndata.computation/ARG_2_LO
   txndata.computation/WCP_RES
   txndata.computation/INST
   )
  )


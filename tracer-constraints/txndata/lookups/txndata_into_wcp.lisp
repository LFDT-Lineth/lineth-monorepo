(module txndata)

(defun   (txn-data-into-wcp-selector)   (* CMPTN computation/WCP_FLAG))

(defclookup
  txndata-into-wcp
  ; target columns
  (
   wcp.ARG_1
   wcp.ARG_2
   wcp.RES
   wcp.INST
   )
  ; source selector
  (txn-data-into-wcp-selector)
  ; source columns
  (
   computation/ARG_1_LO
   computation/ARG_2_LO
   computation/WCP_RES
   computation/INST
 ))


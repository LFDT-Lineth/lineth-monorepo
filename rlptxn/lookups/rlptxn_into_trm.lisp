(defun (sel-rlptxn-to-trm) (* rlptxn.CMP rlptxn.cmp/TRM_FLAG))

(defclookup
  (rlptxn-into-trm :unchecked)
  ;; target columns
  (
    trm.RAW_ADDRESS   
    trm.ADDRESS_HI
  )
  ;; source selector
  (sel-rlptxn-to-trm)
  ;; source columns
  (
   (:: rlptxn.cmp/EXO_DATA_1 rlptxn.cmp/EXO_DATA_2)
   rlptxn.cmp/EXO_DATA_1
  ))

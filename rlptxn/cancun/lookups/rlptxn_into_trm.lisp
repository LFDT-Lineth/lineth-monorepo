(defun (sel-rlptxn-to-trm) (* rlptxn.CMP rlptxn.cmp/TRM_FLAG))

(defclookup
  (rlptxn-into-trm :unchecked)
  ;; target columns
  (
    trm.IOMF
    trm.TRM_ADDRESS_HI
    trm.RAW_ADDRESS_LO
  )
  ;; source selector
  (sel-rlptxn-to-trm)
  ;; source columns
  (
    1
    rlptxn.cmp/EXO_DATA_1   
    rlptxn.cmp/EXO_DATA_2   
  ))

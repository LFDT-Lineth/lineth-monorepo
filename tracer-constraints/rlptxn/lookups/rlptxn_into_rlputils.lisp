(defun (sel-rlptxn-to-rlputils) (* rlptxn.CMP rlptxn.cmp/RLPUTILS_FLAG))

(defclookup
  rlptxn-into-rlputils
  ;; target columns
  (
    rlputils.INST  
    rlputils.DATA_1
    rlputils.DATA_2
    rlputils.DATA_3
    rlputils.DATA_4
    rlputils.DATA_5
    rlputils.DATA_6
    rlputils.DATA_7
    rlputils.DATA_8
  )
  ;; source selector
  (sel-rlptxn-to-rlputils)
  ;; source columns
  (
    rlptxn.cmp/RLPUTILS_INST
    rlptxn.cmp/EXO_DATA_1   
    rlptxn.cmp/EXO_DATA_2   
    rlptxn.cmp/EXO_DATA_3   
    rlptxn.cmp/EXO_DATA_4   
    rlptxn.cmp/EXO_DATA_5   
    rlptxn.cmp/EXO_DATA_6   
    rlptxn.cmp/EXO_DATA_7   
    rlptxn.cmp/EXO_DATA_8   
  ))
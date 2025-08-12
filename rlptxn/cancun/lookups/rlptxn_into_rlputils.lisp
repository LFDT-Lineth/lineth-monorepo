(defun (sel-rlptxn-to-rlputils) (* rlptxn.CMP rlptxn.cmp/RLPUTILS_FLAG))

(defclookup
  rlptxn-into-rlputils
  ;; target columns
  (
    rlputils.MACRO
    rlputils.macro/INST  
    rlputils.macro/DATA_1
    rlputils.macro/DATA_2
    rlputils.macro/DATA_3
    rlputils.macro/DATA_4
    rlputils.macro/DATA_5
    rlputils.macro/DATA_6
    rlputils.macro/DATA_7
    rlputils.macro/DATA_8
  )
  ;; source selector
  (sel-rlptxn-to-rlputils)
  ;; source columns
  (
    1
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
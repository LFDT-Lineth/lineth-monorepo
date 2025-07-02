(defun   (blockhash-into-blockdata-selector)   blockhash.MACRO) ;; ""

(defclookup
  blockhash-into-blockdata
  ;; target columns
  (
   blockdata.REL_BLOCK
   blockdata.DATA_LO
   blockdata.INST
  )
  ;; source selector
  (blockhash-into-blockdata-selector)
  ;; source columns
  (
   blockhash.macro/REL_BLOCK
   blockhash.macro/ABS_BLOCK
   EVM_INST_NUMBER
   )
  )

(defun   (blockhash-into-blockdata-selector)   blockhash.MACRO) ;; ""

(deflookup
  blockhash-into-blockdata
  ; target columns
  (
   blockdata.REL_BLOCK
   blockdata.DATA_LO
   blockdata.INST
   )
  ; source columns
  (
   (* (blockhash-into-blockdata-selector)   blockhash.macro/REL_BLOCK)
   (* (blockhash-into-blockdata-selector)   blockhash.macro/ABS_BLOCK)
   (* (blockhash-into-blockdata-selector)   EVM_INST_NUMBER)
   )
  )

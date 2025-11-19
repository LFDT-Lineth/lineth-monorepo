(defun   (blockhash-into-wcp-selector)   blockhash.PRPRC) ;; ""

(defclookup
  blockhash-into-wcp-lex
  ;; target columns
  (
   wcp.ARG_1
   wcp.ARG_2
   wcp.INST
   wcp.RES
  )
  ;; source selector
  (blockhash-into-wcp-selector)
  ;; source columns
  (
   (:: blockhash.preprocessing/EXO_ARG_1_HI blockhash.preprocessing/EXO_ARG_1_LO)
   (:: blockhash.preprocessing/EXO_ARG_2_HI blockhash.preprocessing/EXO_ARG_2_LO)
   blockhash.preprocessing/EXO_INST
   blockhash.preprocessing/EXO_RES
   )
  )



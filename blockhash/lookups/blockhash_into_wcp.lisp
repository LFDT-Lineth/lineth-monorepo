(defun   (blockhash-into-wcp-selector)   blockhash.PRPRC) ;; ""

(deflookup
  blockhash-into-wcp-lex
  ; target columns
  (
   wcp.ARGUMENT_1_HI
   wcp.ARGUMENT_1_LO
   wcp.ARGUMENT_2_HI
   wcp.ARGUMENT_2_LO
   wcp.INST
   wcp.RESULT
   )
  ; source columns
  (
   (*   blockhash.preprocessing/EXO_ARG_1_HI   (blockhash-into-wcp-selector))
   (*   blockhash.preprocessing/EXO_ARG_1_LO   (blockhash-into-wcp-selector))
   (*   blockhash.preprocessing/EXO_ARG_2_HI   (blockhash-into-wcp-selector))
   (*   blockhash.preprocessing/EXO_ARG_2_LO   (blockhash-into-wcp-selector))
   (*   blockhash.preprocessing/EXO_INST       (blockhash-into-wcp-selector))
   (*   blockhash.preprocessing/EXO_RES        (blockhash-into-wcp-selector))
   )
  )



(defun (hub-into-block-hash-trigger-eip2935-consistency)
  (* hub.PEEK_AT_TRANSACTION
     hub.SYSI
     hub.transaction/EIP_2935
     (- 1 hub.transaction/SYST_TXN_DATA_5))) ;; hub.transaction/SYST_TXN_DATA_5 == is-genesis-block

(defclookup
  hub-into-blockhash-for-eip2935
  ;; target selector
  blockhash.MACRO
  ;; target columns
  (
    blockhash.macro/BLOCKHASH_ARG_HI
    blockhash.macro/BLOCKHASH_ARG_LO
    blockhash.macro/BLOCKHASH_VAL_HI
    blockhash.macro/BLOCKHASH_VAL_LO
  )
  ;; source selector
  (hub-into-block-hash-trigger-eip2935-consistency)
  ;; source columns
  (
    0
    hub.transaction/SYST_TXN_DATA_1                 ;; previous block number (or 0 if genesis)
    hub.transaction/SYST_TXN_DATA_3                 ;; previous blockhash hi (or 0 if genesis)
    hub.transaction/SYST_TXN_DATA_4                 ;; previous blockhash lo (or 0 if genesis)
  ))
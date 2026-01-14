(deflookup
  txndata-into-hub
  ; target columns
  (
   hub.BLK_NUMBER
   hub.TOTL_TXN_NUMBER
   hub.SYSI_TXN_NUMBER
   hub.USER_TXN_NUMBER
   hub.SYSF_TXN_NUMBER
   hub.SYSI
   hub.USER
   hub.SYSF
   )
  ; source columns
  (
   txndata.BLK_NUMBER
   txndata.TOTL_TXN_NUMBER
   txndata.SYSI_TXN_NUMBER
   txndata.USER_TXN_NUMBER
   txndata.SYSF_TXN_NUMBER
   txndata.SYSI
   txndata.USER
   txndata.SYSF
   )
  )



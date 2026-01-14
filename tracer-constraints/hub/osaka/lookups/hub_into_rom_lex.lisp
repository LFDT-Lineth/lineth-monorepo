(defun (hub-into-rom-lex-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/ROMLEX_FLAG))


(defclookup
  (hub-into-romlex :unchecked)
  ;; target columns
  (
   romlex.CODE_FRAGMENT_INDEX
   romlex.CODE_SIZE
   romlex.ADDRESS_HI
   romlex.ADDRESS_LO
   romlex.DEPLOYMENT_NUMBER
   romlex.DEPLOYMENT_STATUS
   romlex.CODE_HASH_HI
   romlex.CODE_HASH_LO
  )
  ;; source selector
  (hub-into-rom-lex-trigger)
  ;; source columns
  (
   hub.account/CODE_FRAGMENT_INDEX
   hub.account/CODE_SIZE_NEW
   hub.account/ADDRESS_HI
   hub.account/ADDRESS_LO
   hub.account/DEPLOYMENT_NUMBER_NEW
   hub.account/DEPLOYMENT_STATUS_NEW
   hub.account/CODE_HASH_HI_NEW
   hub.account/CODE_HASH_LO_NEW
  )
)

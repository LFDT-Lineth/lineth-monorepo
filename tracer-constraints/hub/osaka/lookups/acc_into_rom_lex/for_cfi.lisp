(defun (acc-into-rom-lex-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/ROMLEX_FLAG))


(defclookup
  (hub-into-romlex :unchecked)
  ;; target columns
  (
   romlex.CODE_FRAGMENT_INDEX
   romlex.ADDRESS_HI
   romlex.ADDRESS_LO
   romlex.DEPLOYMENT_NUMBER
   romlex.DEPLOYMENT_STATUS
   romlex.DELEGATION_NUMBER
   romlex.CODE_HASH_HI
   romlex.CODE_HASH_LO
   romlex.CODE_SIZE
   romlex.ACTUALLY_DELEGATION_CODE
   romlex.DELEGATION_ADDRESS_HI
   romlex.DELEGATION_ADDRESS_LO
  )
  ;; source selector
  (acc-into-rom-lex-trigger)
  ;; source columns
  (
   hub.account/CODE_FRAGMENT_INDEX
   hub.account/ADDRESS_HI
   hub.account/ADDRESS_LO
   hub.account/DEPLOYMENT_NUMBER_NEW
   hub.account/DEPLOYMENT_STATUS_NEW
   hub.account/DELEGATION_NUMBER_NEW
   hub.account/CODE_HASH_HI_NEW
   hub.account/CODE_HASH_LO_NEW
   hub.account/CODE_SIZE_NEW
   hub.account/IS_DELEGATED_NEW
   hub.account/DELEGATION_ADDRESS_HI_NEW
   hub.account/DELEGATION_ADDRESS_LO_NEW
  )
)

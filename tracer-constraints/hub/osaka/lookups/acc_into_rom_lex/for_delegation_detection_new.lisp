(defun (trigger-delegation-check-new)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/CHECK_FOR_DELEGATION_NEW))


(defclookup
  (hub-into-romlex---delegation-detection---new :unchecked)
  ;; target columns
  (
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
  ;;
  ;; source selector
  ;;
  (trigger-delegation-check-new)
  ;; source columns
  (
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



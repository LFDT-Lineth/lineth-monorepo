(defun (trigger-delegation-check)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/CHECK_FOR_DELEGATION))


(defclookup
  (hub-into-romlex---delegation-detection :unchecked)
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
  (trigger-delegation-check)
  ;; source columns
  (
   hub.account/ADDRESS_HI
   hub.account/ADDRESS_LO
   hub.account/DEPLOYMENT_NUMBER
   hub.account/DEPLOYMENT_STATUS
   hub.account/DELEGATION_NUMBER
   hub.account/CODE_HASH_HI
   hub.account/CODE_HASH_LO
   hub.account/CODE_SIZE
   hub.account/IS_DELEGATED
   hub.account/DELEGATION_ADDRESS_HI
   hub.account/DELEGATION_ADDRESS_LO
  )
)



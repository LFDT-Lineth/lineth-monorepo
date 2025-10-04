(defun (hub-into-trm-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/TRM_FLAG))

(defclookup hub-into-trm
  ;; target columns
  (
   trm.RAW_ADDRESS
   trm.ADDRESS_HI
   trm.IS_PRECOMPILE
  )
  ;; source selector
  (hub-into-trm-trigger)
  ;; source columns
  (
   (:: hub.account/TRM_RAW_ADDRESS_HI hub.account/ADDRESS_LO) 
   hub.account/ADDRESS_HI
   hub.account/IS_PRECOMPILE
  )
)

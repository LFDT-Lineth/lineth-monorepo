(defun (hub-into-trm-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/TRM_FLAG))

(defclookup hub-into-trm
  ;; target columns
  (
   trm.TRM_ADDRESS_HI
   trm.RAW_ADDRESS_HI
   trm.RAW_ADDRESS_LO
   trm.IS_PRECOMPILE
  )
  ;; source selector
  (hub-into-trm-trigger)
  ;; source columns
  (
   hub.account/ADDRESS_HI
   hub.account/TRM_RAW_ADDRESS_HI
   hub.account/ADDRESS_LO
   hub.account/IS_PRECOMPILE
  )
)

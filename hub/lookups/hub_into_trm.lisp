(defun (hub-into-trm-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/TRM_FLAG))

(deflookup hub-into-trm
           ;; target columns
           (
             trm.TRM_ADDRESS_HI
             trm.RAW_ADDRESS_HI
             trm.RAW_ADDRESS_LO
             trm.IS_PRECOMPILE
             )
           ;; source columns
           (
            (* hub.account/ADDRESS_HI                    (hub-into-trm-trigger))
            (* hub.account/TRM_RAW_ADDRESS_HI            (hub-into-trm-trigger))
            (* hub.account/ADDRESS_LO                    (hub-into-trm-trigger))
            (* hub.account/IS_PRECOMPILE                 (hub-into-trm-trigger))
            )
           )

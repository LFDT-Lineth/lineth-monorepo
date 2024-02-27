(defun (hub-into-trm-trigger)
  (and hub_v2.PEEK_AT_ACCOUNT
       hub_v2.account/TRM_FLAG))

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
            (* hub_v2.account/ADDRESS_HI                    (hub-into-trm-trigger))
            (* hub_v2.account/TRM_RAW_ADDR_HI               (hub-into-trm-trigger))
            (* hub_v2.account/ADDRESS_LO                    (hub-into-trm-trigger))
            (* hub_v2.account/IS_PRECOMPILE                 (hub-into-trm-trigger))
            )
           )

(defun    (hub-into-hub-source-selector)    hub.scp_PEEK_AT_STORAGE)
(defun    (hub-into-hub-target-selector)    hub.acp_PEEK_AT_ACCOUNT) ;; ""

(defclookup hub-into-hub---FIRST-FINAL-in-block-deployment-number-coherence
  ;; target selector
  (hub-into-hub-target-selector)
  ;; target columns
  (
   hub.acp_ADDRESS_HI
   hub.acp_ADDRESS_LO
   hub.acp_BLK_NUMBER
   hub.acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK
   hub.acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK
   hub.acp_EXISTS_FIRST_IN_BLOCK
   hub.acp_EXISTS_FINAL_IN_BLOCK
  )
  ;; source selector
  (hub-into-hub-source-selector)
  ;; source columns
  (
   hub.scp_ADDRESS_HI
   hub.scp_ADDRESS_LO
   hub.scp_BLK_NUMBER
   hub.scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK
   hub.scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK
   hub.scp_EXISTS_FIRST_IN_BLOCK
   hub.scp_EXISTS_FINAL_IN_BLOCK
  )
)


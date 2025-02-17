(defun    (hub-into-hub-source-selector)    hub.scp_PEEK_AT_STORAGE)
(defun    (hub-into-hub-target-selector)    hub.acp_PEEK_AT_ACCOUNT) ;; ""

(deflookup hub-into-hub---FIRST-FINAL-in-block-deployment-number-coherence
	   ;; target columns
	   (
	    (* hub.acp_ADDRESS_HI                                 (hub-into-hub-target-selector))
	    (* hub.acp_ADDRESS_LO                                 (hub-into-hub-target-selector))
	    (* hub.acp_REL_BLK_NUM                                (hub-into-hub-target-selector))
	    (* hub.acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK           (hub-into-hub-target-selector))
	    (* hub.acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK           (hub-into-hub-target-selector))
	    )
	   ;; source columns
	   (
	    (* hub.scp_ADDRESS_HI                                 (hub-into-hub-source-selector))
	    (* hub.scp_ADDRESS_LO                                 (hub-into-hub-source-selector))
	    (* hub.scp_REL_BLK_NUM                                (hub-into-hub-source-selector))
	    (* hub.scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK           (hub-into-hub-source-selector))
	    (* hub.scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK           (hub-into-hub-source-selector))
	    )
	   )


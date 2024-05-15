(deflookup hub-into-hub-for-DEP_NUM_INFTY-coherence
           ;; target columns
	   ( 
	     (* hub.account/ADDRESS_HI                         hub.PEEK_AT_ACCOUNT)
	     (* hub.account/ADDRESS_HI                         hub.PEEK_AT_ACCOUNT)
	     (* hub.account/DEPLOYMENT_NUMBER_INFTY            hub.PEEK_AT_ACCOUNT)
           )
           ;; source columns
	   (
	     (* hub.storage/ADDRESS_HI                         hub.PEEK_AT_STORAGE)
	     (* hub.storage/ADDRESS_HI                         hub.PEEK_AT_STORAGE)
	     (* hub.storage/DEPLOYMENT_NUMBER_INFTY            hub.PEEK_AT_STORAGE)
           )
)


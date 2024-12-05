(defun (hub-into-rom-lex-trigger)
  (and hub.PEEK_AT_ACCOUNT
       hub.account/ROMLEX_FLAG))


(deflookup hub-into-romlex
           ;; target columns
	   (
	     romlex.CODE_FRAGMENT_INDEX
	     romlex.CODE_SIZE
	     romlex.ADDRESS_HI
	     romlex.ADDRESS_LO
	     romlex.DEPLOYMENT_NUMBER
	     romlex.DEPLOYMENT_STATUS
	     romlex.CODE_HASH_HI
	     romlex.CODE_HASH_LO
           )
           ;; source columns
	   (
	     (* hub.account/CODE_FRAGMENT_INDEX      (hub-into-rom-lex-trigger))
	     (* hub.account/CODE_SIZE_NEW            (hub-into-rom-lex-trigger))
	     (* hub.account/ADDRESS_HI               (hub-into-rom-lex-trigger))
	     (* hub.account/ADDRESS_LO               (hub-into-rom-lex-trigger))
	     (* hub.account/DEPLOYMENT_NUMBER_NEW    (hub-into-rom-lex-trigger))
	     (* hub.account/DEPLOYMENT_STATUS_NEW    (hub-into-rom-lex-trigger))
	     (* hub.account/CODE_HASH_HI_NEW         (hub-into-rom-lex-trigger))
	     (* hub.account/CODE_HASH_LO_NEW         (hub-into-rom-lex-trigger))
           )
)

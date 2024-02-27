(defun (hub-into-rom-lex-trigger)
  (and hub_v2.PEEK_AT_ACCOUNT
       hub_v2.account/ROM_LEX_FLAG)) ;; TODO: the selector is different in the spec

(deflookup hub-into-rom-lex
           ;; target columns
	   ( 
	     romLex.CODE_FRAGMENT_INDEX
	     romLex.ADDRESS_HI
	     romLex.ADDRESS_LO       
	     romLex.DEPLOYMENT_NUMBER
	     romLex.DEPLOYMENT_STATUS
           )
           ;; source columns
	   (
	     (* hub_v2.account/CODE_FRAGMENT_INDEX                (hub-into-rom-lex-trigger))
	     (* hub_v2.account/ADDRESS_HI                         (hub-into-rom-lex-trigger))
	     (* hub_v2.account/ADDRESS_LO                         (hub-into-rom-lex-trigger))
	     (* hub_v2.account/DEPLOYMENT_NUMBER                  (hub-into-rom-lex-trigger))
	     (* hub_v2.account/DEPLOYMENT_STATUS                  (hub-into-rom-lex-trigger))
           )
)

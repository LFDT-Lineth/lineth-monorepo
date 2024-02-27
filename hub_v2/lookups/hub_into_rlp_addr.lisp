(defun (hub-into-rlpAddr-trigger) 
  (and hub_v2.PEEK_AT_ACCOUNT
       hub_v2.account/RLPADDR_FLAG))

;;
(deflookup hub-into-rlpAddr
           ;; target columns
	   ( 
	     rlpAddr.RECIPE
	     rlpAddr.ADDR_HI
	     rlpAddr.ADDR_LO
	     rlpAddr.NONCE
	     rlpAddr.DEP_ADDR_HI
	     rlpAddr.DEP_ADDR_LO
	     rlpAddr.SALT_HI
	     rlpAddr.SALT_LO
	     rlpAddr.KEC_HI
	     rlpAddr.KEC_LO
           )
           ;; source columns
	   (
	     (* hub_v2.account/RLPADDR_RECIPE                         (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/ADDRESS_HI                             (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/ADDRESS_LO                             (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/NONCE                                  (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_DEP_ADDR_HI                    (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_DEP_ADDR_LO                    (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_SALT_HI                        (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_SALT_LO                        (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_KEC_HI                         (hub-into-rlpAddr-trigger))
	     (* hub_v2.account/RLPADDR_KEC_LO                         (hub-into-rlpAddr-trigger))
           )
)

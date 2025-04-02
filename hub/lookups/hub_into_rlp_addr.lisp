(defun (hub-into-rlp-addr-trigger)
  (* hub.PEEK_AT_ACCOUNT
     hub.account/RLPADDR_FLAG))

;;
(deflookup hub-into-rlpaddr
           ;; target columns
	   (
	     rlpaddr.RECIPE
	     rlpaddr.ADDR_HI
	     rlpaddr.ADDR_LO
	     rlpaddr.NONCE
	     rlpaddr.DEP_ADDR_HI
	     rlpaddr.DEP_ADDR_LO
	     rlpaddr.SALT_HI
	     rlpaddr.SALT_LO
	     rlpaddr.KEC_HI
	     rlpaddr.KEC_LO
           )
           ;; source columns
	   (
	     (* hub.account/RLPADDR_RECIPE                         (hub-into-rlp-addr-trigger))
	     (* hub.account/ADDRESS_HI                             (hub-into-rlp-addr-trigger))
	     (* hub.account/ADDRESS_LO                             (hub-into-rlp-addr-trigger))
	     (* hub.account/NONCE                                  (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_DEP_ADDR_HI                    (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_DEP_ADDR_LO                    (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_SALT_HI                        (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_SALT_LO                        (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_KEC_HI                         (hub-into-rlp-addr-trigger))
	     (* hub.account/RLPADDR_KEC_LO                         (hub-into-rlp-addr-trigger))
           )
)

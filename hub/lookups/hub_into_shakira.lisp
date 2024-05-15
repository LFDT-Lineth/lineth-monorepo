(defun (hub-into-shakira-trigger)
  (*
    hub.PEEK_AT_STACK
    hub.stack/HASH_INFO_FLAG))

(deflookup hub-into-shakira
	   ;; target columns
	   (
	     shakiradata.PHASE
	     shakiradata.ID
	     shakiradata.INDEX
	     ;; shakiradata.TOTAL_SIZE
	     (shift shakiradata.LIMB -1)
	     shakiradata.LIMB
	     )
	   ;; source columns
	   (
	    (* PHASE_KECCAK_RESULT            (hub-into-shakira-trigger))
	    (* (+ 1 hub.HUB_STAMP)            (hub-into-shakira-trigger))
	    (* 1                              (hub-into-shakira-trigger)) ;; we could just write (hub-into-shakira-trigger)
	    ;; (* hub.stack/HASH_INFO_SIZE       (hub-into-shakira-trigger))
	    (* hub.stack/HASH_INFO_KECCAK_HI     (hub-into-shakira-trigger))
	    (* hub.stack/HASH_INFO_KECCAK_LO     (hub-into-shakira-trigger))
	    )
	   )

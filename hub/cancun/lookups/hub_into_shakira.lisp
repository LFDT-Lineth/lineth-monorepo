(defun (hub-into-shakira-trigger)
  (*
    hub.PEEK_AT_STACK
    hub.stack/HASH_INFO_FLAG))

(defclookup
  (hub-into-shakiradata :unchecked)
  ;; target columns
  (
   shakiradata.PHASE
   shakiradata.ID
   shakiradata.INDEX
   (shift shakiradata.LIMB -1)
   shakiradata.LIMB
  )
  ;; source selector
  (hub-into-shakira-trigger)
  ;; source columns
  (
   PHASE_KECCAK_RESULT
   (+ 1 hub.HUB_STAMP)
   1
   hub.stack/HASH_INFO_KECCAK_HI
   hub.stack/HASH_INFO_KECCAK_LO
  )
)

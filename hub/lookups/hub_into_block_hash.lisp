(defun (hub-into-block-hash-trigger)
  (* hub.PEEK_AT_STACK (- 1 hub.XAHOY) hub.stack/BTC_FLAG [hub.stack/DEC_FLAG 1]))

(deflookup
  hub-into-blockhash
  ;; target columns
  (
    blockhash.REL_BLOCK
    blockhash.BLOCK_NUMBER_HI
    blockhash.BLOCK_NUMBER_LO
    blockhash.RES_HI
    blockhash.RES_LO
  )
  ;; source columns
  (
    (*  hub.RELATIVE_BLOCK_NUMBER        (hub-into-block-hash-trigger))
    (* [hub.stack/STACK_ITEM_VALUE_HI 1] (hub-into-block-hash-trigger))
    (* [hub.stack/STACK_ITEM_VALUE_LO 1] (hub-into-block-hash-trigger))
    (* [hub.stack/STACK_ITEM_VALUE_HI 4] (hub-into-block-hash-trigger))
    (* [hub.stack/STACK_ITEM_VALUE_LO 4] (hub-into-block-hash-trigger))
  ))



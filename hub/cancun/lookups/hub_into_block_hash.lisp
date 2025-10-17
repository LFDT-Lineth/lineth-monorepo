(defun (hub-into-block-hash-trigger)
  (* hub.PEEK_AT_STACK (- 1 hub.XAHOY) hub.stack/BTC_FLAG [hub.stack/DEC_FLAG 1]))

(defclookup
  hub-into-blockhash
  ;; target selector
  blockhash.MACRO
  ;; target columns
  (
    blockhash.macro/REL_BLOCK
    blockhash.macro/BLOCKHASH_ARG_HI
    blockhash.macro/BLOCKHASH_ARG_LO
    blockhash.macro/BLOCKHASH_RES_HI
    blockhash.macro/BLOCKHASH_RES_LO
  )
  ;; source selector
  (hub-into-block-hash-trigger)
  ;; source columns
  (
     hub.BLK_NUMBER
    [hub.stack/STACK_ITEM_VALUE_HI 1]
    [hub.stack/STACK_ITEM_VALUE_LO 1]
    [hub.stack/STACK_ITEM_VALUE_HI 4]
    [hub.stack/STACK_ITEM_VALUE_LO 4]
  ))



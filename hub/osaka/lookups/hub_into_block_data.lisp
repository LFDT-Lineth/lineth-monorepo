(defun (hub-into-block-data-trigger) (* hub.PEEK_AT_STACK
                                        (- 1 hub.XAHOY)
                                        hub.stack/BTC_FLAG
                                        [hub.stack/DEC_FLAG 2]))

(defclookup
  (hub-into-blockdata :unchecked)
  ;; target columns
  (
   blockdata.REL_BLOCK
   blockdata.INST
   blockdata.DATA_HI
   blockdata.DATA_LO
  )
  ;; source selector
  (hub-into-block-data-trigger)
  ;; source columns
  (
   hub.BLK_NUMBER
   hub.stack/INSTRUCTION
   [hub.stack/STACK_ITEM_VALUE_HI 4]
   [hub.stack/STACK_ITEM_VALUE_LO 4]
  )
)

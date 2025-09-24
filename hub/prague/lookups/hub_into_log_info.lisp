(defun (hub-into-log-info-trigger)
  (* hub.PEEK_AT_STACK hub.stack/LOG_INFO_FLAG (- 1 hub.CT_TLI)))

(defclookup
  (hub-into-loginfo :unchecked)
  ;; target columns
  (
    loginfo.ABS_TXN_NUM
    loginfo.ABS_LOG_NUM
    loginfo.INST
    loginfo.ADDR_HI
    loginfo.ADDR_LO
    [loginfo.TOPIC_HI 1]
    [loginfo.TOPIC_LO 1]
    [loginfo.TOPIC_HI 2]
    [loginfo.TOPIC_LO 2]
    [loginfo.TOPIC_HI 3]
    [loginfo.TOPIC_LO 3]
    [loginfo.TOPIC_HI 4]
    [loginfo.TOPIC_LO 4]
    loginfo.DATA_SIZE
  )
  ;; source selector
  (hub-into-log-info-trigger)
  ;; source columns
  (
    hub.USER_TXN_NUMBER
    hub.LOG_INFO_STAMP
    hub.stack/INSTRUCTION
    (shift hub.context/ACCOUNT_ADDRESS_HI 2)
    (shift hub.context/ACCOUNT_ADDRESS_LO 2)
    (next [hub.stack/STACK_ITEM_VALUE_HI 1])
    (next [hub.stack/STACK_ITEM_VALUE_LO 1])
    (next [hub.stack/STACK_ITEM_VALUE_HI 2])
    (next [hub.stack/STACK_ITEM_VALUE_LO 2])
    (next [hub.stack/STACK_ITEM_VALUE_HI 3])
    (next [hub.stack/STACK_ITEM_VALUE_LO 3])
    (next [hub.stack/STACK_ITEM_VALUE_HI 4])
    (next [hub.stack/STACK_ITEM_VALUE_LO 4])
    [hub.stack/STACK_ITEM_VALUE_LO 2]
  ))



(defun (hub-into-block-data-trigger) (* hub.PEEK_AT_STACK
                                        (- 1 hub.XAHOY)
                                        hub.stack/BTC_FLAG
                                        [hub.stack/DEC_FLAG 2]))

(deflookup hub-into-blockdata
           ;; target columns
           (
             blockdata.REL_BLOCK
             blockdata.INST
             blockdata.DATA_HI
             blockdata.DATA_LO
             )
           ;; source columns
           (
            (* hub.RELATIVE_BLOCK_NUMBER          (hub-into-block-data-trigger))
            (* hub.stack/INSTRUCTION              (hub-into-block-data-trigger))
            (* [hub.stack/STACK_ITEM_VALUE_HI 4]  (hub-into-block-data-trigger))
            (* [hub.stack/STACK_ITEM_VALUE_LO 4]  (hub-into-block-data-trigger))
            )
           )

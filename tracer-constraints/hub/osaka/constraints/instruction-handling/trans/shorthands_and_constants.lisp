(module hub)


(defun    (transient-storage-instruction---no-stack-exceptions)          (* PEEK_AT_STACK stack/TRANS_FLAG (- 1 stack/SUX stack/SOX)))
(defun    (transient-storage-instruction---is-TLOAD)                     [ stack/DEC_FLAG 1 ])
(defun    (transient-storage-instruction---is-TSTORE)                    [ stack/DEC_FLAG 2 ])
(defun    (transient-storage-instruction---transient-key-hi)             [ stack/STACK_ITEM_VALUE_HI 1 ])
(defun    (transient-storage-instruction---transient-key-lo)             [ stack/STACK_ITEM_VALUE_LO 1 ])
(defun    (transient-storage-instruction---value-to-tload-hi)            [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (transient-storage-instruction---value-to-tload-lo)            [ stack/STACK_ITEM_VALUE_LO 4 ])
(defun    (transient-storage-instruction---value-to-tstore-hi)           [ stack/STACK_ITEM_VALUE_HI 4 ])
(defun    (transient-storage-instruction---value-to-tstore-lo)           [ stack/STACK_ITEM_VALUE_LO 4 ]) ;; ""


(defconst

  ;; row offsets for exceptional instructions
  TRANSIENT___EXCEPTIONAL___CONTEXT_CURRENT___ROFF   1
  TRANSIENT___EXCEPTIONAL___CONTEXT_PARENT___ROFF    2

  ;; row offsets for unexceptional instructions
  TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF      1
  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF   2
  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_UNDOING_ROFF 3
  )

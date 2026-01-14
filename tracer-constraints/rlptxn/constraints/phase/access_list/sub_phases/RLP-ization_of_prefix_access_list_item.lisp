(module rlptxn)

(defconstraint    access-list---rlp-prefix-of-access-list-items---RLP_UTILS-calls ()
                  (if-not-zero (* IS_PREFIX_OF_ACCESS_LIST_ITEM (prev DONE))
                               (rlp-compound-constraint---BYTE_STRING_PREFIX-non-trivial   0
                                                                                           (rlptxn---access-list---AL-item-RLP-length-countdown)
                                                                                           1
                                                                                           1)))

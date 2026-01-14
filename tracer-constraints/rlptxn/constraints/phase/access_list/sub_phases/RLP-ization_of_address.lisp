(module rlptxn)

(defconstraint    access-list---RLP-ization-of-addresses---RLP_UTILS-calls ()
                  (if-not-zero    (* IS_ACCESS_LIST_ADDRESS (prev DONE))
                                  (rlp-compound-constraint---ADDRESS    0
                                                                        (rlptxn---access-list---address-hi)
                                                                        (rlptxn---access-list---address-lo))))

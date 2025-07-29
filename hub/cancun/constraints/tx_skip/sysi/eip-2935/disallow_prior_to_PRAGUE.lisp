(module hub)


;; TODO: delete file for PRAGUE


(defconstraint    tx-skip---SYSI-2935---disallow-these-system-transactions-prior-to-PRAGUE
                  (:guard (tx-skip---precondition---SYSI-2935))
                  (vanishes!    (shift    transaction/EIP_2935    tx-skip---SYSI-2935---row-offset---TXN)))

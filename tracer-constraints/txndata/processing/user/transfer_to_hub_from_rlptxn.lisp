(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X. USER transaction processing      ;;
;;    X.Y Data transfer HUB -from- RLP    ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---is-deployment-nonce-value-and-gas-limit
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!   (USER-transaction---HUB---is-deployment)   (USER-transaction---RLP---is-deployment))
                   (eq!   (USER-transaction---HUB---value)           (USER-transaction---RLP---value))
                   (eq!   (USER-transaction---HUB---nonce)           (USER-transaction---RLP---nonce))
                   (eq!   (USER-transaction---HUB---gas-limit)       (USER-transaction---RLP---gas-limit))
                   ))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---call-data-size-and-init-code-size
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (eq!   (USER-transaction---HUB---call-data-size)   (*   (USER-transaction---RLP---is-message-call)   (USER-transaction---payload-size)))
                   (eq!   (USER-transaction---HUB---init-code-size)   (*   (USER-transaction---RLP---is-deployment)     (USER-transaction---payload-size)))
                   ))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---conditionally-setting-the-gas-price
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (USER-transaction---tx-decoding---tx-type-with-fixed-gas-price)
                                (eq!   (USER-transaction---HUB---gas-price)
                                       (USER-transaction---RLP---gas-price))))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---conditionally-set-the-address
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (USER-transaction---RLP---is-message-call)
                                (begin
                                  (eq!   (shift   hub/TO_ADDRESS_HI   ROFF___USER___HUB_ROW)   (USER-transaction---RLP---to-address-hi-or-zero))
                                  (eq!   (shift   hub/TO_ADDRESS_LO   ROFF___USER___HUB_ROW)   (USER-transaction---RLP---to-address-lo-or-zero))
                                  )))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---marking-transactions-supporting-EIP-1550-gas-semantics
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (USER-transaction---HUB---transaction-supports-eip-1559-gas-semantics)
                        (USER-transaction---tx-decoding---tx-type-sans-fixed-gas-price)
                        ))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---marking-transactions-supporting-delegation-lists
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   (USER-transaction---HUB---transaction-supports-delegation-lists)
                        (USER-transaction---tx-decoding---tx-type-with-delegation)
                        ))

(defconstraint   USER-transaction---data-transfer---HUB-from-RLP---transfering-length-of-delegation-list
                 (:guard   (first-row-of-USER-transaction))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq! (shift   rlp/LENGTH_OF_DELEGATION_LIST   ROFF___USER___RLP_ROW)
                      (shift   hub/LENGTH_OF_DELEGATION_LIST   ROFF___USER___HUB_ROW)))

(defproperty    USER-transaction---data-transfer---HUB-from-RLP---zeroing-delegation-counts-when-no-delegations
                ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                (if-not-zero   (first-row-of-USER-transaction)
                               (if-not-zero   (USER-transaction---tx-decoding---tx-type-sans-delegation)
                                              (begin
                                                (vanishes!   (shift   hub/LENGTH_OF_DELEGATION_LIST                 ROFF___USER___HUB_ROW))
                                                (vanishes!   (shift   hub/NUMBER_OF_SUCCESSFUL_SENDER_DELEGATIONS   ROFF___USER___HUB_ROW))
                                                ))))

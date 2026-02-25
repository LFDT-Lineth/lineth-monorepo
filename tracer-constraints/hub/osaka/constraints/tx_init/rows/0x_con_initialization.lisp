(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X     TX_INIT phase        ;;
;;   X.Y   Common constraints   ;;
;;   X.Y.Z Transaction row      ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun    (tx-init---initialize-execution-context   row-offset)
  (begin   (initialize-context    row-offset
                                  CONTEXT_NUMBER_NEW                                                                                    ;; context number
                                  0                                                                                                     ;; call stack depth
                                  1                                                                                                     ;; is root
                                  0                                                                                                     ;; is static
                                  (tx-init---recipient-address-hi)                                                                      ;; account address high
                                  (tx-init---recipient-address-lo)                                                                      ;; account address low
                                  (shift     account/DEPLOYMENT_NUMBER_NEW     tx-init---row-offset---ACC---recipient-value-reception)  ;; account deployment number
                                  (shift     account/ADDRESS_HI                tx-init---row-offset---ACC---delegate-reading)           ;; byte code address high
                                  (shift     account/ADDRESS_LO                tx-init---row-offset---ACC---delegate-reading)           ;; byte code address low
                                  (shift     account/DEPLOYMENT_NUMBER_NEW     tx-init---row-offset---ACC---delegate-reading)           ;; byte code deployment number
                                  (shift     account/DEPLOYMENT_STATUS_NEW     tx-init---row-offset---ACC---delegate-reading)           ;; byte code deployment status
                                  (shift     account/CODE_FRAGMENT_INDEX       tx-init---row-offset---ACC---delegate-reading)           ;; byte code code fragment index
                                  (tx-init---sender-address-hi)                                                                         ;; caller address high
                                  (tx-init---sender-address-lo)                                                                         ;; caller address low
                                  (tx-init---value)                                                                                     ;; call value
                                  (tx-init---call-data-context-number)                                                                  ;; caller context
                                  0                                                                                                     ;; call data offset
                                  (tx-init---call-data-size)                                                                            ;; call data size
                                  0                                                                                                     ;; return at offset
                                  0                                                                                                     ;; return at capacity
                                  )))

(defconstraint    tx-init---initializing-execution-context---failure
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-failure-prediction)
                                  (tx-init---initialize-execution-context    tx-init---row-offset---CON---context-initialization-row---failure)))

(defconstraint    tx-init---initializing-execution-context---success
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (tx-init---transaction-success-prediction)
                                  (tx-init---initialize-execution-context    tx-init---row-offset---CON---context-initialization-row---success)))

(defconstraint    tx-init---CONTEXT_NUMBER_NEW-sanity-check
                  (:guard (tx-init---standard-precondition))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq! CONTEXT_NUMBER_NEW (+ 1 HUB_STAMP)))

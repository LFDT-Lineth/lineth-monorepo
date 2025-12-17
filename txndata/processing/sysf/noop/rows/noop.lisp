(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                      ;;
;;    X. SYSF transaction processing    ;;
;;    X.Y Generalities                  ;;
;;                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;



(defproperty   SYSF-prelude-constraints---noop-transactions-require-no-computations
               (if-not-zero (first-row-of-SYSF-transaction)
                            (eq!    (+   (shift   computation/EUC_FLAG    ROFF___SYSF___CMP_ROW)
                                         (shift   computation/WCP_FLAG    ROFF___SYSF___CMP_ROW))
                                    0)))


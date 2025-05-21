(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   3.6 Constraints for the hub stamp   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint final-row (:domain {-1})
               (if-not-zero ABS_TX_NUM
                            (eq! (+ TX_SKIP TX_FINL PEEK_AT_TRANSACTION) 2)))

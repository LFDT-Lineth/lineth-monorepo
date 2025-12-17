(module rlptxn)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                      ;;
;;    X.Y.Z.T access_list_item_countdown constraints    ;;
;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint rlptxn---access-list-item-countdown---initialization ()
               (if-not-zero (is-access-list-prefix)
                            (eq! (rlptxn---access-list---access-list-item-countdown)
                                 (prev txn/NUMBER_OF_PREWARMED_ADDRESSES))))

(defun   (rlptxn---access-list---new-access-list-item)   (* IS_PREFIX_OF_ACCESS_LIST_ITEM
                                                            (prev DONE)))

(defconstraint rlptxn---access-list-item-countdown---update ()
               (if-not-zero (is-access-list-data)
                            (did-dec! (rlptxn---access-list---access-list-item-countdown)
                                      (rlptxn---access-list---new-access-list-item))))


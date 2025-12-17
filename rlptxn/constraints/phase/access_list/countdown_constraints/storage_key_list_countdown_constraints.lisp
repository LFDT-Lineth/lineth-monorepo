(module rlptxn)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                      ;;
;;    X.Y.Z.T storage_key_list_countdown constraints    ;;
;;                                                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    rlptxn---access-list---storage-key-list-countdown---update ()
                  (if-not-zero    IS_ACCESS_LIST_STORAGE_KEY
                                  (did-dec! (rlptxn---access-list---storage-key-list-countdown)
                                            (prev DONE))))

(defconstraint    rlptxn---access-list---storage-key-list-countdown---finalization ()
                  (if-not-zero    (end-of-tuple-or-end-of-phase)
                                  (vanishes! (rlptxn---access-list---storage-key-list-countdown))))

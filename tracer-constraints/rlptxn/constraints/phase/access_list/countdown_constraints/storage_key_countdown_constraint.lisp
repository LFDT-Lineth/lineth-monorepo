(module rlptxn)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                 ;;
;;    X.Y.Z.T storage_key_countdown constraints    ;;
;;                                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   rlptxn---access-list---storage-key-countdown---initialization ()
                 (if-not-zero   (is-access-list-prefix)
                                (eq!   (rlptxn---access-list---storage-key-countdown)
                                       (prev txn/NUMBER_OF_PREWARMED_STORAGE_KEYS))))

(defun   (start-RLP-izing-new-storage-key)   (* IS_ACCESS_LIST_STORAGE_KEY
                                                (prev DONE)))

(defconstraint   rlptxn---access-list---storage-key-countdown---update ()
                 (if-not-zero   (is-access-list-data)
                                (did-dec!   (rlptxn---access-list---storage-key-countdown)
                                            (start-RLP-izing-new-storage-key))))

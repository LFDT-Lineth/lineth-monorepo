(module rlptxn)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                            ;;
;;    X.Y.Z.T address constancy conditions    ;;
;;                                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; address constancy
(defconstraint   access-list---address-constancy-condition ()
                 (if-not-zero (force-bin (+ IS_ACCESS_LIST_ADDRESS
                                            IS_PREFIX_OF_STORAGE_KEY_LIST
                                            IS_ACCESS_LIST_STORAGE_KEY))
                              (begin
                                (remained-constant! (rlptxn---access-list---address-hi))
                                (remained-constant! (rlptxn---access-list---address-lo))))) 


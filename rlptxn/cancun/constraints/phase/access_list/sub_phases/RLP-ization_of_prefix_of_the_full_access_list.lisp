(module rlptxn)

(defconstraint   access-list---rlp-prefix-of-the-full-access-list---RLP_UTILS-call
                 (:guard   (is-access-list-prefix))
                 (rlp-compound-constraint---BYTE_STRING_PREFIX-non-trivial    0
                                                                              (rlptxn---access-list---AL-RLP-length-countdown)
                                                                              1
                                                                              0))


(defconstraint   access-list---rlp-prefix-of-the-full-access-list---setting-PHASE_END
                 (:guard   (is-access-list-prefix))
                 (eq!   PHASE_END
                        (access-list-is-empty)))

(defun (access-list-is-non-empty)        cmp/EXO_DATA_4) ;; ""
(defun (access-list-is-empty)            (- 1 (access-list-is-non-empty)))

(defproperty    access-list---rlp-prefix-of-the-full-access-list---sanity-checks
                 (if-not-zero (is-access-list-prefix)
                 (if-zero  PHASE_END
                           (begin
                             (eq!   (next IS_PREFIX_OF_ACCESS_LIST_ITEM)   1)
                             (eq!   (next IS_ACCESS_LIST_ADDRESS)          0)
                             (eq!   (next IS_PREFIX_OF_STORAGE_KEY_LIST)   0)
                             (eq!   (next IS_ACCESS_LIST_STORAGE_KEY)      0)))))

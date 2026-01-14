(module rlptxn)


(defun    (access-list---first-row-of-storage-key-list-processing)    (* IS_PREFIX_OF_STORAGE_KEY_LIST
                                                                         (prev DONE)))

(defconstraint    access-list---RLP-prefix-for-storage-key-lists---calling-RLP_UTILS
                  (:guard    (access-list---first-row-of-storage-key-list-processing))
                  (rlp-compound-constraint---BYTE_STRING_PREFIX-non-trivial    0
                                                                               (length-of-concatenated-storage-key-RLPs)
                                                                               1
                                                                               0))

(defun (length-of-concatenated-storage-key-RLPs)    (* 33 (rlptxn---access-list---storage-key-list-countdown)))
(defun (storage-key-list-is-nonempty)               (force-bin cmp/EXO_DATA_4))
(defun (storage-key-list-is-empty)                  (force-bin (- 1 (storage-key-list-is-nonempty)))) ;; ""

(defconstraint    access-list---RLP-prefix-for-storage-key-lists---setting-the-next-step-I
                  (:guard    (access-list---first-row-of-storage-key-list-processing))
                  (eq!   (next IS_ACCESS_LIST_STORAGE_KEY)
                         (storage-key-list-is-nonempty)))

(defconstraint    access-list---RLP-prefix-for-storage-key-lists---setting-the-next-step-II
                  (:guard    (access-list---first-row-of-storage-key-list-processing))
                  (if-not-zero    (storage-key-list-is-empty)
                                  (if-not-zero (rlptxn---access-list---access-list-item-countdown)
                                               (eq! (next IS_PREFIX_OF_ACCESS_LIST_ITEM) 1)
                                               (eq! PHASE_END 1))))



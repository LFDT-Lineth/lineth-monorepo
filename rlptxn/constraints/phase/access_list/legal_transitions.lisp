(module rlptxn)

(defconstraint   access-list---setting-first-row-after-prefix ()
                 (if-not-zero (is-access-list-prefix)
                              (eq! (next IS_PREFIX_OF_ACCESS_LIST_ITEM) (next IS_ACCESS_LIST))))

(defconstraint   access-list---setting-flag-after-rlp-tuple ()
                 (if-not-zero IS_PREFIX_OF_ACCESS_LIST_ITEM
                              (begin
                                (eq! (+ (next IS_PREFIX_OF_ACCESS_LIST_ITEM)
                                        (next IS_ACCESS_LIST_ADDRESS))
                                     1)
                                (eq! (next IS_ACCESS_LIST_ADDRESS)
                                     DONE))))

(defconstraint   access-list---setting-flag-after-address ()
                 (if-not-zero IS_ACCESS_LIST_ADDRESS
                              (begin
                                (eq! (+ (next IS_ACCESS_LIST_ADDRESS)
                                        (next IS_PREFIX_OF_STORAGE_KEY_LIST))
                                     1)
                                (eq! (next IS_PREFIX_OF_STORAGE_KEY_LIST)
                                     DONE))))

(defconstraint   access-list---setting-flag-after-storage-list-list-rlp ()
                 (if-not-zero IS_PREFIX_OF_STORAGE_KEY_LIST
                              (begin
                                (eq! (+ (next IS_PREFIX_OF_STORAGE_KEY_LIST)
                                        (next IS_ACCESS_LIST_STORAGE_KEY)
                                        (next IS_PREFIX_OF_ACCESS_LIST_ITEM))
                                     (next IS_ACCESS_LIST))
                                (eq! (+ (next IS_ACCESS_LIST_STORAGE_KEY)
                                        (next IS_PREFIX_OF_ACCESS_LIST_ITEM))
                                     (* (next IS_ACCESS_LIST) DONE)))))

(defconstraint   access-list---setting-flag-after-storage-key ()
                 (if-not-zero IS_ACCESS_LIST_STORAGE_KEY
                              (begin
                                (eq! (+ (next IS_ACCESS_LIST_STORAGE_KEY)
                                        (next IS_PREFIX_OF_ACCESS_LIST_ITEM))
                                     (next IS_ACCESS_LIST))
                                (if-not-zero (next IS_PREFIX_OF_ACCESS_LIST_ITEM)
                                             (eq! DONE 1)))))

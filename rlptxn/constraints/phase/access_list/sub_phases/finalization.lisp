(module rlptxn)

(defconstraint   access-list---finalization-constraints ()
    (if-not-zero (* IS_ACCESS_LIST PHASE_END)
        (begin
        (vanishes! (rlptxn---access-list---access-list-item-countdown))
        (vanishes! (rlptxn---access-list---storage-key-countdown))
        (vanishes! (rlptxn---access-list---storage-key-list-countdown))
        (vanishes! (rlptxn---access-list---AL-RLP-length-countdown))
        (vanishes! (rlptxn---access-list---AL-item-RLP-length-countdown))
        )))


(defproperty   access-list---finish-either-on-prefix-or-storage-key-list-or-storage-key
    (if-not-zero (* IS_ACCESS_LIST PHASE_END)
               (eq!   (+   (is-access-list-prefix)
                           IS_PREFIX_OF_STORAGE_KEY_LIST
                           IS_ACCESS_LIST_STORAGE_KEY)
                      1)))

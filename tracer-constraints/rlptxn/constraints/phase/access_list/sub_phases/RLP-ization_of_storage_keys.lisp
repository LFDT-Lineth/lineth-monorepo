(module rlptxn)


(defun    (access-list---first-row-of-storage-key-processing)    (* IS_ACCESS_LIST_STORAGE_KEY
                                                                    (prev DONE)))


(defconstraint access-list---rlp-ization-of-storage-keys---calling-RLP_UTILS
               (:guard   (access-list---first-row-of-storage-key-processing))
               (rlp-compound-constraint---BYTES32    0
                                                     (rlptxn---access-list---storage-hi)
                                                     (rlptxn---access-list---storage-lo)))


(defun (rlptxn---access-list---storage-hi) cmp/EXO_DATA_1)
(defun (rlptxn---access-list---storage-lo) cmp/EXO_DATA_2) ;; ""


(defconstraint access-list---rlp-ization-of-storage-keys---setting-the-next-step
               (:guard   (access-list---first-row-of-storage-key-processing))
               (let ((ROFF_CURR_FINAL   2)  ;; row offset of final row of current IS_ACCESS_LIST_STORAGE_KEY-cycle
                     (ROFF_NEXT_START   3)) ;; row offset of first row of next    cycle
                 (if-not-zero (rlptxn---access-list---storage-key-list-countdown)
                              (eq! (shift IS_ACCESS_LIST_STORAGE_KEY                 ROFF_NEXT_START) 1)
                              (if-not-zero (rlptxn---access-list---access-list-item-countdown)
                                           (eq! (shift IS_PREFIX_OF_ACCESS_LIST_ITEM ROFF_NEXT_START) 1)
                                           (eq! (shift PHASE_END                     ROFF_CURR_FINAL) 1)))))


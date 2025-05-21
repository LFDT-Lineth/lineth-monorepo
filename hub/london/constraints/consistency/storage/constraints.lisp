(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.6 Storage consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; we should be guaranteed that this is a 20B integer given how it is filled:
;; - during pre-warming addresses are checked for smallness in the RLP_TXN
;; - or during SSTORE / SSLOAD operations addresses are obtained from context data
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (scp_full_address) (+ (* (^ 256 16) scp_ADDRESS_HI)
                             scp_ADDRESS_LO)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    X.6.3 Constraints for scp_FIRST, scp_AGAIN and scp_FINAL   ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; TODO: remove when we migrate to the unified permutation argument
(defconstraint storage-consistency---FIRST-AGAIN-FINAL---automatic-vanishing ()
               (begin
                 (if-zero (force-bin scp_PEEK_AT_STORAGE)
                          (vanishes! (+
                                       scp_FIRST_IN_TXN   scp_FIRST_IN_BLK   scp_FIRST_IN_CNF
                                       scp_AGAIN_IN_TXN   scp_AGAIN_IN_BLK   scp_AGAIN_IN_CNF
                                       scp_FINAL_IN_TXN   scp_FINAL_IN_BLK   scp_FINAL_IN_CNF)))))

(defun    (storage-consistency---transtion-conflation)    (+    (prev scp_FINAL_IN_CNF)    scp_FIRST_IN_CNF))
(defun    (storage-consistency---transtion-block)         (+    (prev scp_FINAL_IN_BLK)    scp_FIRST_IN_BLK))
(defun    (storage-consistency---transtion-transaction)   (+    (prev scp_FINAL_IN_TXN)    scp_FIRST_IN_TXN))
(defun    (storage-consistency---transtion-sum)           (+    (storage-consistency---transtion-conflation)
                                                                (storage-consistency---transtion-block)
                                                                (storage-consistency---transtion-transaction)))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---first-storage-row ()
                  (if-zero    (force-bin      (prev scp_PEEK_AT_STORAGE))
                              (if-not-zero    (force-bin    scp_PEEK_AT_STORAGE)
                                              (if-not-zero   scp_PEEK_AT_STORAGE
                                                             (eq!    (storage-consistency---transtion-sum)
                                                                     3)))))

(defun   (storage-consistency---repeat-storage-row)    (*    (prev    scp_PEEK_AT_STORAGE)   scp_PEEK_AT_STORAGE))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---repeat-storage-row---change-in-storage-slot  (:guard   (storage-consistency---repeat-storage-row))
                  (begin
                    (if-not (remained-constant!   (scp_full_address))    (eq! (storage-consistency---transtion-sum)   6))
                    (if-not (remained-constant!   scp_STORAGE_KEY_HI)    (eq! (storage-consistency---transtion-sum)   6))
                    (if-not (remained-constant!   scp_STORAGE_KEY_LO)    (eq! (storage-consistency---transtion-sum)   6))))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---repeat-storage-row---no-change-in-storage-slot  (:guard   (storage-consistency---repeat-storage-row))
                  (if (remained-constant!   (scp_full_address))
                      (if (remained-constant!   scp_STORAGE_KEY_HI)
                          (if (remained-constant!   scp_STORAGE_KEY_LO)
                              (eq! (storage-consistency---transtion-conflation) 0)))))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---repeat-encounter-of-storage-slot    (:guard    (*   scp_PEEK_AT_STORAGE    (-   1   scp_FIRST_IN_CNF)))
                  (begin
                    (if-not    (remained-constant!    scp_REL_BLK_NUM)
                               (eq!    (storage-consistency---transtion-block)   2)
                               (eq!    (storage-consistency---transtion-block)   0))
                    (if-not    (remained-constant!    scp_ABS_TX_NUM)
                               (eq!    (storage-consistency---transtion-transaction)   2)
                               (eq!    (storage-consistency---transtion-transaction)   0))))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---final-row-with-room-to-spare ()
                  (if-not-zero (prev scp_PEEK_AT_STORAGE)
                               (if-zero    (force-bin    scp_PEEK_AT_STORAGE)
                                           (eq!    3
                                                   (storage-consistency---transtion-sum)))))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---final-row-of-the-trace       (:domain {-1})
                  (if-not-zero scp_PEEK_AT_STORAGE
                               (eq!    3
                                       (+   scp_FINAL_IN_CNF
                                            scp_FINAL_IN_BLK
                                            scp_FINAL_IN_TXN))))

(defconstraint    storage-consistency---FIRST-AGAIN-FINAL---unconditionally-constraining-AGAIN ()
                  (begin
                    (eq!   (+   scp_FIRST_IN_CNF   scp_AGAIN_IN_CNF)   scp_PEEK_AT_STORAGE)
                    (eq!   (+   scp_FIRST_IN_BLK   scp_AGAIN_IN_BLK)   scp_PEEK_AT_STORAGE)
                    (eq!   (+   scp_FIRST_IN_TXN   scp_AGAIN_IN_TXN)   scp_PEEK_AT_STORAGE)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.6.7 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint storage-consistency---setting-original-storage-value (:guard   scp_FIRST_IN_TXN)
               (begin
                 (eq!   scp_VALUE_ORIG_HI   scp_VALUE_CURR_HI)
                 (eq!   scp_VALUE_ORIG_LO   scp_VALUE_CURR_LO)))

(defconstraint storage-consistency---persisting-original-storage-values (:guard   scp_AGAIN_IN_TXN)
               (begin
                 (remained-constant!   scp_VALUE_ORIG_HI)
                 (remained-constant!   scp_VALUE_ORIG_LO)))

(defconstraint storage-consistency---resetting-of-storage-values-after-deployments---first (:guard   scp_FIRST_IN_CNF)
               (if-not-zero scp_DEPLOYMENT_NUMBER
                            (begin
                              (vanishes! scp_VALUE_CURR_HI)
                              (vanishes! scp_VALUE_CURR_LO))))

(defconstraint storage-consistency---resetting-of-storage-values-after-deployments---again (:guard   scp_AGAIN_IN_CNF)
               (if-not   (remained-constant!   scp_DEPLOYMENT_NUMBER)
                         (begin
                            (vanishes! scp_VALUE_CURR_HI)
                            (vanishes! scp_VALUE_CURR_LO))))

(defconstraint storage-consistency---persisting-of-storage-values (:guard   scp_AGAIN_IN_CNF)
               (if   (remained-constant!   scp_DEPLOYMENT_NUMBER)
                     (begin
                       (eq!   scp_VALUE_CURR_HI   (prev   scp_VALUE_NEXT_HI))
                       (eq!   scp_VALUE_CURR_LO   (prev   scp_VALUE_NEXT_LO)))))

(defconstraint setting-and-resetting-storage-key-warmth ()
               (begin
                 (if-not-zero scp_FIRST_IN_TXN (vanishes! scp_WARMTH))
                 (if-not-zero scp_AGAIN_IN_TXN (eq!       scp_WARMTH    (prev    scp_WARMTH_NEW)))))

(defconstraint exclusivity-and-sanity-checks-for-_OPERATION-columns ()
               (if-not-zero    scp_PEEK_AT_STORAGE
                               (begin
                                 (eq!         1
                                              (+  scp_PREWARMING_OPERATION
                                                  scp_SLOAD_OPERATION
                                                  scp_SSTORE_OPERATION))
                                 (vanishes!   (*  scp_PREWARMING_OPERATION   scp_EXCEPTIONAL_OPERATION)))))

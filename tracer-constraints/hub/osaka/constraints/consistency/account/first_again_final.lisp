(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;    X.5.1 Properties of the permutation   ;;
;;    X.5.2 Permuted columns                ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; we are guaranteed that this is a 20B integer
(defun (acp_full_address) (+ (* (^ 256 16) acp_ADDRESS_HI)
                             acp_ADDRESS_LO)) ;; ""

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                               ;;
;;    X.5.3 Constraints for acp_FIRST, acp_AGAIN and acp_FINAL   ;;
;;                                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint account-consistency---FIRST-AGAIN-FINAL---automatic-vanishing ()
               (begin
                 (if-zero (force-bin acp_PEEK_AT_ACCOUNT)
                          (vanishes! (+
                                       acp_FIRST_IN_TXN   acp_FIRST_IN_BLK   acp_FIRST_IN_CNF
                                       acp_AGAIN_IN_TXN   acp_AGAIN_IN_BLK   acp_AGAIN_IN_CNF
                                       acp_FINAL_IN_TXN   acp_FINAL_IN_BLK   acp_FINAL_IN_CNF)))))

(defun    (account-consistency---transition-conflation)    (+    (prev acp_FINAL_IN_CNF)    acp_FIRST_IN_CNF))
(defun    (account-consistency---transition-block)         (+    (prev acp_FINAL_IN_BLK)    acp_FIRST_IN_BLK))
(defun    (account-consistency---transition-transaction)   (+    (prev acp_FINAL_IN_TXN)    acp_FIRST_IN_TXN))
(defun    (account-consistency---transition-sum)           (+    (account-consistency---transition-conflation)
                                                                (account-consistency---transition-block)
                                                                (account-consistency---transition-transaction)))


(defconstraint    account-consistency---FIRST-AGAIN-FINAL---first-account-row ()
                  (if-zero    (force-bin      (prev acp_PEEK_AT_ACCOUNT))
                              (if-not-zero    (force-bin    acp_PEEK_AT_ACCOUNT)
                                              (if-not-zero   acp_PEEK_AT_ACCOUNT
                                                             (eq!    (account-consistency---transition-sum)
                                                                     3)))))

(defun   (account-consistency---repeat-account-row)    (*    (prev    acp_PEEK_AT_ACCOUNT)   acp_PEEK_AT_ACCOUNT))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---conflation-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not  (remained-constant! (acp_full_address))
                           (eq! (account-consistency---transition-conflation) 2)
                           (eq! (account-consistency---transition-conflation) 0)))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---block-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not (remained-constant!   (acp_full_address))              (eq! (account-consistency---transition-block) 2))
                    (if-not (remained-constant!    acp_BLK_NUMBER)                 (eq! (account-consistency---transition-block) 2))
                    (if     (remained-constant!   (acp_full_address))
                            (if    (remained-constant!    acp_BLK_NUMBER)    (eq! (account-consistency---transition-block) 0)))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---transaction-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not (remained-constant!   (acp_full_address))              (eq! (account-consistency---transition-transaction) 2))
                    (if-not (remained-constant!    acp_TOTL_TXN_NUMBER)            (eq! (account-consistency---transition-transaction) 2))
                    (if     (remained-constant!   (acp_full_address))
                            (if    (remained-constant!    acp_TOTL_TXN_NUMBER)     (eq! (account-consistency---transition-transaction) 0)))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---final-row-with-room-to-spare ()
                  (if-not-zero (prev acp_PEEK_AT_ACCOUNT)
                               (if-zero    (force-bin    acp_PEEK_AT_ACCOUNT)
                                           (eq!    3
                                                   (account-consistency---transition-sum)))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---final-row-of-the-trace       (:domain {-1})
                  (if-not-zero acp_PEEK_AT_ACCOUNT
                               (eq!    3
                                       (+   acp_FINAL_IN_CNF
                                            acp_FINAL_IN_BLK
                                            acp_FINAL_IN_TXN))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---unconditionally-constraining-AGAIN ()
                  (begin
                    (eq!   (+   acp_FIRST_IN_CNF   acp_AGAIN_IN_CNF)   acp_PEEK_AT_ACCOUNT)
                    (eq!   (+   acp_FIRST_IN_BLK   acp_AGAIN_IN_BLK)   acp_PEEK_AT_ACCOUNT)
                    (eq!   (+   acp_FIRST_IN_TXN   acp_AGAIN_IN_TXN)   acp_PEEK_AT_ACCOUNT)))


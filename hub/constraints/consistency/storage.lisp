(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.6 Storage consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defpermutation 
  ;; permuted columns
  ;; replace scp with storage_consistency_permutation
  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
  (
    scp_PEEK_AT_STORAGE
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_STORAGE_KEY_HI
    scp_STORAGE_KEY_LO
    scp_DOM_STAMP
    scp_SUB_STAMP
    ;;
    scp_ABS_TX_NUM
    scp_VALUE_ORIG_HI
    scp_VALUE_ORIG_LO
    scp_VALUE_CURR_HI
    scp_VALUE_CURR_LO
    scp_VALUE_NEXT_HI
    scp_VALUE_NEXT_LO
    ;;
    scp_WARMTH
    scp_WARMTH_NEW
    scp_DEPLOYMENT_NUMBER
    scp_DEPLOYMENT_NUMBER_INFTY
  )
  ;; original columns
  ;;;;;;;;;;;;;;;;;;;
  (
    (↓ PEEK_AT_STORAGE )
    (↓ storage/ADDRESS_HI )
    (↓ storage/ADDRESS_LO )
    (↓ storage/STORAGE_KEY_HI )
    (↓ storage/STORAGE_KEY_LO )
    (↓ DOM_STAMP )
    (↑ SUB_STAMP )
    ;;
    ABS_TX_NUM
    storage/VALUE_ORIG_HI
    storage/VALUE_ORIG_LO
    storage/VALUE_CURR_HI
    storage/VALUE_CURR_LO
    storage/VALUE_NEXT_HI
    storage/VALUE_NEXT_LO
    ;;
    storage/WARMTH
    storage/WARMTH_NEW
    storage/DEPLOYMENT_NUMBER
    storage/DEPLOYMENT_NUMBER_INFTY
  )
)

;; we should be guaranteed that this is a 20B integer given how it is filled:
;; - during pre-warming addresses are checked for smallness in the RLP_TXN
;; - or during SSTORE / SSLOAD operations addresses are obtained from context data
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
(defun (scp_full_address) (+ (* (^ 256 16) scp_ADDRESS_HI)
                             scp_ADDRESS_LO))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                        ;;
;;    X.6.3 Constraints for FIRST/FINAL   ;;
;;                                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint sto-FIRST-FINAL-are-binary ()
               (begin
                 (is-binary sto_FIRST)
                 (is-binary sto_FINAL)))

;; TODO: remove when we migrate to the unified permutation argument
(defconstraint sto-FIRST-FINAL-trivial ()
               (if-not-zero (- 1 scp_PEEK_AT_STORAGE)
                            (vanishes! (+ sto_FIRST sto_FINAL))))

(defconstraint sto-FIRST-FINAL-initialization ()
               (if-not-zero (prev (- 1 scp_PEEK_AT_STORAGE))
                            (if-not-zero scp_PEEK_AT_STORAGE
                                         (eq! 1 sto_FIRST))))

(defconstraint sto-FIRST-FINAL-régime-de-croisière ()
               (if-not-zero (prev scp_PEEK_AT_STORAGE)
                            (if-not-zero scp_PEEK_AT_STORAGE
                                         (begin
                                           (if-not-zero (remained-constant! (scp_full_address))
                                                        (begin
                                                          (was-eq! sto_FINAL 1)
                                                          (eq!     sto_FIRST 1)))
                                           (if-not-zero (remained-constant! scp_STORAGE_KEY_HI)
                                                        (begin
                                                          (was-eq! sto_FINAL 1)
                                                          (eq!     sto_FIRST 1)))
                                           (if-not-zero (remained-constant! scp_STORAGE_KEY_LO)
                                                        (begin
                                                          (was-eq! sto_FINAL 1)
                                                          (eq!     sto_FIRST 1)))
                                           (if-zero (remained-constant! (scp_full_address))
                                                    (if-zero (remained-constant! scp_STORAGE_KEY_HI)
                                                             (if-zero (remained-constant! scp_STORAGE_KEY_LO)
                                                                      (begin
                                                                        (vanishes! (prev sto_FINAL))
                                                                        (vanishes!       sto_FIRST)))))))))

(defconstraint sto-FIRST-FINAL-final-storage-row-1 ()
               (if-not-zero (prev scp_PEEK_AT_STORAGE)
                            (if-not-zero (- 1 scp_PEEK_AT_STORAGE)
                                         (eq! (prev sto_FINAL) 1))))

;; TODO: remove when we migrate to the unified permutation argument
(defconstraint sto-FIRST-FINAL-final-storage-row-2 (:domain {-1})
               (if-not-zero scp_PEEK_AT_STORAGE
                            (vanishes! sto_FINAL)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;;    X.6.4 Constraints   ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint setting-original-storage-value ()
               (if-not-zero sto_FIRST
                            (begin
                              (eq! scp_VALUE_ORIG_HI scp_VALUE_CURR_HI)
                              (eq! scp_VALUE_ORIG_LO scp_VALUE_CURR_LO))))

(defconstraint perpetuating-and-resetting-original-storage-value ()
               (if-not-zero scp_PEEK_AT_STORAGE
                            (if-zero sto_FIRST
                                     (if-eq-else scp_ABS_TX_NUM (prev scp_ABS_TX_NUM)
                                                 (begin
                                                   (remained-constant! scp_VALUE_ORIG_HI)
                                                   (remained-constant! scp_VALUE_ORIG_LO))
                                                 (begin
                                                   (eq!  scp_VALUE_ORIG_HI  scp_VALUE_CURR_HI)
                                                   (eq!  scp_VALUE_ORIG_LO  scp_VALUE_CURR_LO))))))

(defconstraint setting-and-resetting-storage-value ()
               (if-not-zero sto_FIRST
                            ;; FIRST = 1
                            (if-not-zero scp_DEPLOYMENT_NUMBER
                                         (begin
                                           (vanishes! scp_VALUE_CURR_HI)
                                           (vanishes! scp_VALUE_CURR_LO)))
                            ;; FIRST = 0
                            (if-not-zero scp_PEEK_AT_STORAGE
                                         (if-not-zero (remained-constant! scp_DEPLOYMENT_NUMBER)
                                                      (begin
                                                        (vanishes! scp_VALUE_CURR_HI)
                                                        (vanishes! scp_VALUE_CURR_LO))
                                                      (begin
                                                        (was-eq! scp_VALUE_NEXT_HI scp_VALUE_CURR_HI)
                                                        (was-eq! scp_VALUE_NEXT_LO scp_VALUE_CURR_LO))))))

(defconstraint setting-and-resetting-storage-key-warmth ()
               (if-not-zero sto_FIRST
                            ;; FIRST = 1
                            (vanishes! scp_WARMTH)
                            ;; FIRST = 0
                            (if-not-zero scp_PEEK_AT_STORAGE
                                         (if-eq-else scp_ABS_TX_NUM (prev scp_ABS_TX_NUM)
                                                     (was-eq!   scp_WARMTH_NEW scp_WARMTH)
                                                     (vanishes! scp_WARMTH)))))

(defconstraint perpetuating-the-final-deployment-number ()
               (if-not-zero scp_PEEK_AT_STORAGE
                            (if-zero sto_FIRST
                                     (remained-constant! scp_DEPLOYMENT_NUMBER_INFTY))))

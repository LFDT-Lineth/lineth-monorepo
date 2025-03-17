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
                 (if-zero (force-bool acp_PEEK_AT_ACCOUNT)
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
                  (if-zero    (force-bool     (prev acp_PEEK_AT_ACCOUNT))
                              (if-not-zero    (force-bool    acp_PEEK_AT_ACCOUNT)
                                              (if-not-zero   acp_PEEK_AT_ACCOUNT
                                                             (eq!    (account-consistency---transition-sum)
                                                                     3)))))

(defun   (account-consistency---repeat-account-row)    (*    (prev    acp_PEEK_AT_ACCOUNT)   acp_PEEK_AT_ACCOUNT))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---conflation-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero  (remained-constant! (acp_full_address))
                                (eq! (account-consistency---transition-conflation) 2)
                                (eq! (account-consistency---transition-conflation) 0)))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---block-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero (remained-constant!   (acp_full_address))              (eq! (account-consistency---transition-block) 2))
                    (if-not-zero (remained-constant!    acp_REL_BLK_NUM)                (eq! (account-consistency---transition-block) 2))
                    (if-zero     (remained-constant!   (acp_full_address))
                                 (if-zero    (remained-constant!    acp_REL_BLK_NUM)    (eq! (account-consistency---transition-block) 0)))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---repeat-encounter---transaction-level
                  (:guard   (account-consistency---repeat-account-row))
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (if-not-zero (remained-constant!   (acp_full_address))              (eq! (account-consistency---transition-transaction) 2))
                    (if-not-zero (remained-constant!    acp_ABS_TX_NUM)                 (eq! (account-consistency---transition-transaction) 2))
                    (if-zero     (remained-constant!   (acp_full_address))
                                 (if-zero    (remained-constant!    acp_ABS_TX_NUM)     (eq! (account-consistency---transition-transaction) 0)))))

(defconstraint    account-consistency---FIRST-AGAIN-FINAL---final-row-with-room-to-spare ()
                  (if-not-zero (prev acp_PEEK_AT_ACCOUNT)
                               (if-zero    (force-bool    acp_PEEK_AT_ACCOUNT)
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

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;    X.5.4 Initialization Constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---initialization---conflation-level  (:guard   acp_FIRST_IN_CNF)
                  (begin
                    (eq!        acp_TRM_FLAG    1)
                    (vanishes!  acp_DEPLOYMENT_NUMBER)))

(defconstraint    account-consistency---initialization---block-level       (:guard   acp_FIRST_IN_BLK)
                  (begin
                    (eq!    acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK    acp_DEPLOYMENT_NUMBER)
                    (eq!    acp_EXISTS_FIRST_IN_BLOCK               acp_EXISTS           )
                    ))

(defconstraint    account-consistency---initialization---transaction-level (:guard   acp_FIRST_IN_TXN)
                  (begin
                    (eq!        acp_WARMTH    acp_IS_PRECOMPILE)
                    (vanishes!  acp_DEPLOYMENT_STATUS)
                    (vanishes!  acp_MARKED_FOR_SELFDESTRUCT)))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.5.5 Linking Constraints   ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;-----------------------------;
;    X.5.5 Conflation level   ;
;-----------------------------;

(defconstraint    account-consistency---linking---conflation-level---nonce
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_NONCE                     (prev acp_NONCE_NEW)               ))

(defconstraint    account-consistency---linking---conflation-level---balance
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_BALANCE                   (prev acp_BALANCE_NEW)             ))

(defconstraint    account-consistency---linking---conflation-level---code
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!          acp_CODE_SIZE          (prev acp_CODE_SIZE_NEW)           )
                    (eq!          acp_CODE_HASH_HI       (prev acp_CODE_HASH_HI_NEW)        )
                    (eq!          acp_CODE_HASH_LO       (prev acp_CODE_HASH_LO_NEW)        )
                    (debug (eq!   acp_EXISTS             (prev acp_EXISTS_NEW)              ))))

(defconstraint    account-consistency---linking---conflation-level---precompile-status
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (eq!   acp_IS_PRECOMPILE             (prev acp_IS_PRECOMPILE)           ))

(defconstraint    account-consistency---linking---conflation-level---deployment-number-and-status
                  (:guard   acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   acp_DEPLOYMENT_NUMBER         (prev acp_DEPLOYMENT_NUMBER_NEW))
                    (eq!   acp_DEPLOYMENT_STATUS         (prev acp_DEPLOYMENT_STATUS_NEW))
                    ))


;------------------------;
;    X.5.5 Block level   ;
;------------------------;

(defconstraint    account-consistency---linking---block-level
                  (:guard   acp_AGAIN_IN_BLK)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (remained-constant!    acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK)
                    (remained-constant!    acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK)
                    (remained-constant!    acp_EXISTS_FIRST_IN_BLOCK)
                    (remained-constant!    acp_EXISTS_FINAL_IN_BLOCK)
                    ))


;------------------------------;
;    X.5.5 Transaction level   ;
;------------------------------;

(defconstraint    account-consistency---linking---transaction-level
                  (:guard   acp_AGAIN_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!   acp_WARMTH                     (prev    acp_WARMTH_NEW))
                    (eq!   acp_MARKED_FOR_SELFDESTRUCT    (prev    acp_MARKED_FOR_SELFDESTRUCT_NEW))
                    (if-not-zero    acp_MARKED_FOR_SELFDESTRUCT
                                    (eq!    acp_MARKED_FOR_SELFDESTRUCT_NEW    1))))

(defconstraint    account-consistency---linking---for-CFI
                  (:guard    acp_AGAIN_IN_CNF)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-eq    acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER
                            (if-eq    acp_DEPLOYMENT_STATUS_NEW    acp_DEPLOYMENT_STATUS
                                      (remained-constant!    acp_CODE_FRAGMENT_INDEX))))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                     ;;
;;    X.5.6 Finalization Constraints   ;;
;;                                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---finalization---block-level
                  (:guard   acp_FINAL_IN_BLK)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (begin
                    (eq!    acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK    acp_DEPLOYMENT_NUMBER_NEW)
                    (eq!    acp_EXISTS_FINAL_IN_BLOCK               acp_EXISTS_NEW           )
                    ))

(defconstraint    account-consistency---finalization---transaction-level
                  (:guard   acp_FINAL_IN_TXN)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (vanishes!    acp_DEPLOYMENT_STATUS_NEW))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.5.7 Other Constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---other---monotony-of-deployment-number
                  (:guard    acp_PEEK_AT_ACCOUNT)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (any!    (eq!   acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER)
                           (eq!   acp_DEPLOYMENT_NUMBER_NEW    (+    1    acp_DEPLOYMENT_NUMBER))))


(defconstraint    account-consistency---other---vanishing-constraints-upon-trivial-deployments
                  (:guard    acp_PEEK_AT_ACCOUNT)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (-    acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER)
                                  (if-zero   acp_DEPLOYMENT_STATUS_NEW
                                             (begin
                                               ;; current account state
                                               (vanishes!   acp_DEPLOYMENT_STATUS)
                                               ;; updated account state
                                               (vanishes!   acp_CODE_SIZE_NEW)
                                               (eq!         acp_CODE_HASH_HI_NEW    EMPTY_KECCAK_HI)
                                               (eq!         acp_CODE_HASH_LO_NEW    EMPTY_KECCAK_LO)))))

(defconstraint    account-consistency---other---vanishing-constraints-upon-nontrivial-deployments
                  (:guard    acp_PEEK_AT_ACCOUNT)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (if-not-zero    (-    acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER)
                                  (if-not-zero   acp_DEPLOYMENT_STATUS_NEW
                                                 (begin
                                                   ;; current account state
                                                   (vanishes!   acp_NONCE)
                                                   (vanishes!   acp_CODE_SIZE)
                                                   (eq!         acp_CODE_HASH_HI    EMPTY_KECCAK_HI)
                                                   (eq!         acp_CODE_HASH_LO    EMPTY_KECCAK_LO)
                                                   (vanishes!   acp_DEPLOYMENT_STATUS)
                                                   ;; updated account state
                                                   (eq!         acp_NONCE_NEW    1)
                                                   (eq!         acp_CODE_HASH_HI_NEW    EMPTY_KECCAK_HI)
                                                   (eq!         acp_CODE_HASH_LO_NEW    EMPTY_KECCAK_LO)))))

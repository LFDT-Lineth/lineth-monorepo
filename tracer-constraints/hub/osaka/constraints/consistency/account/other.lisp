(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                          ;;;;
;;;;    X.5 Account consistency constraints   ;;;;
;;;;                                          ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;    X.5.7 Other Constraints   ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---other---monotony-of-deployment-number
                  (:guard    acp_PEEK_AT_ACCOUNT)
                  ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                  (or!    (eq!   acp_DEPLOYMENT_NUMBER_NEW    acp_DEPLOYMENT_NUMBER)
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

(defconstraint account-consistency---other---vanishing-deletion-for-account-already-having-code
               (:guard    acp_PEEK_AT_ACCOUNT)
               ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
               (if-not-zero acp_HAD_CODE_INITIALLY
                            (begin
                              (vanishes! acp_MARKED_FOR_DELETION     )
                              (vanishes! acp_MARKED_FOR_DELETION_NEW )))
               )

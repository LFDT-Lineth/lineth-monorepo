(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                      ;;
;;   X.1 Introduction   ;;
;;   X.2 Constraints    ;;
;;                      ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint   prewarming-phase---peeking-into-either-account-or-storage                (:guard TX_WARM)
                 (eq! (+ PEEK_AT_ACCOUNT PEEK_AT_STORAGE) 1))

(defconstraint   prewarming-phase---trivial-dom-sub-stamps              (:guard TX_WARM)
                 (DOM-SUB-stamps---standard     0
                                                0))

(defconstraint   prewarming-phase---first-prewarming-row-peeks-into-account        (:guard TX_WARM)
                 (if-zero (prev TX_WARM)
                          (eq! PEEK_AT_ACCOUNT 1)))

(defconstraint   prewarming-phase---perpetuating-address-and-deployment-number-for-storage-rows     (:guard (* TX_WARM (prev TX_WARM) PEEK_AT_STORAGE))
                 (begin
                   (if-not-zero (prev PEEK_AT_ACCOUNT)
                                (begin
                                  (eq! storage/ADDRESS_HI        (prev account/ADDRESS_HI))
                                  (eq! storage/ADDRESS_LO        (prev account/ADDRESS_LO))
                                  (eq! storage/DEPLOYMENT_NUMBER (prev account/DEPLOYMENT_NUMBER))))
                   (if-not-zero (prev PEEK_AT_STORAGE)
                                (begin
                                  (remained-constant! storage/ADDRESS_HI)
                                  (remained-constant! storage/ADDRESS_LO)
                                  (remained-constant! storage/DEPLOYMENT_NUMBER)))))

(defconstraint   prewarming-phase---turn-on-warmth-on-account-rows                          (:guard TX_WARM)
                 (if-not-zero PEEK_AT_ACCOUNT
                              (begin
                                (account-same-balance                               0)
                                (account-same-nonce                                 0)
                                (account-same-code                                  0)
                                (account-same-deployment-number-and-status          0)
                                (account-turn-on-warmth                             0)
                                (account-same-marked-for-deletion                   0)
                                (debug (account-trim-address                        0
                                                                                    account/ADDRESS_HI
                                                                                    account/ADDRESS_LO)))))

(defconstraint   prewarming-phase---turn-on-warmth-on-storage-rows                          (:guard TX_WARM)
                 (if-not-zero PEEK_AT_STORAGE
                              (begin
                                (storage-reading                                      0)
                                (storage-turn-on-warmth                               0))))

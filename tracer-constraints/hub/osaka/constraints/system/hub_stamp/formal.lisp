(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X.Y The HUB_STAMP column   ;;
;;   X.Y.Z Formal properties    ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defproperty    system-hub-stamp---formal-properties
                (begin
                  (if-not-zero    TX_SKIP                                       (did-inc!     HUB_STAMP    TXN                                           ))
                  (if-not-zero    TX_AUTH                                       (did-inc!     HUB_STAMP    (+ PEEK_AT_AUTHORIZATION PEEK_AT_TRANSACTION) ))
                  (if-not-zero    TX_INIT                                       (did-inc!     HUB_STAMP    TXN                                           ))
                  (if-not-zero    TX_FINL                                       (did-inc!     HUB_STAMP    TXN                                           ))
                  (if-not-zero    TX_EXEC                                       (did-inc!     HUB_STAMP    (*  (- 1 CT_TLI)  PEEK_AT_STACK)              ))
                  (if-not         (will-remain-constant!    BLK_NUMBER)         (will-inc!    HUB_STAMP    1                                             ))
                  (if-not         (will-remain-constant!    TOTL_TXN_NUMBER)    (will-inc!    HUB_STAMP    1                                             ))
                  (if-not-zero    (+    (zero-now-one-next    TX_SKIP)
                                        (zero-now-one-next    TX_WARM)
                                        (zero-now-one-next    TX_AUTH)
                                        (zero-now-one-next    TX_INIT)
                                        (zero-now-one-next    TX_FINL)
                                        (zero-now-one-next    TX_EXEC))
                                  (will-inc!    HUB_STAMP    1))
                  ))

(defproperty    system-hub-stamp---formal-properties---TX_AUTH-special-edition
                (if-not-zero   TX_AUTH   (did-inc!   HUB_STAMP   (+ PEEK_AT_AUTHORIZATION PEEK_AT_TRANSACTION) )
                               ))

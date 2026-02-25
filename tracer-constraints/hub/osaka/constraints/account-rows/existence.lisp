(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                       ;;
;;   X.3 Account existence constraints   ;;
;;                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   account---existence-curr (:perspective account)
                 (begin (debug (is-binary account/EXISTS))
                        (if-not-zero account/NONCE       (eq! account/EXISTS 1))
                        (if-not-zero account/BALANCE     (eq! account/EXISTS 1))
                        (if-not-zero account/HAS_CODE    (eq! account/EXISTS 1))
                        (if-zero account/NONCE
                                 (if-zero account/BALANCE
                                          (if-zero account/HAS_CODE (eq! account/EXISTS 0))))))

(defconstraint   account---existence-next (:perspective account)
                 (begin (debug (is-binary account/EXISTS_NEW))
                        (if-not-zero account/NONCE_NEW    (eq! account/EXISTS_NEW 1))
                        (if-not-zero account/BALANCE_NEW  (eq! account/EXISTS_NEW 1))
                        (if-not-zero account/HAS_CODE_NEW (eq! account/EXISTS_NEW 1))
                        (if-zero account/NONCE_NEW
                                 (if-zero account/BALANCE_NEW
                                          (if-zero account/HAS_CODE_NEW (eq! account/EXISTS_NEW 0))))))


(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                              ;;
;;   X.Y The HUB_STAMP column   ;;
;;   X.Y.Z Increments           ;;
;;                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint    system---hub-stamp-increments---at-block-transitions                    ()    (if-not         (will-remain-constant!   BLK_NUMBER) (will-inc!    HUB_STAMP    1)))
(defconstraint    system---hub-stamp-increments---at-the-end-of-TX_SKIP                   ()    (if-not-zero    TX_SKIP                              (will-inc!    HUB_STAMP    CON)))
(defconstraint    system---hub-stamp-increments---at-every-step-of-TX_WARM                ()    (if-not-zero    TX_WARM                              (will-inc!    HUB_STAMP    1)))
(defconstraint    system---hub-stamp-increments---at-every-delegation-tuple-of-TX_AUTH    ()    (if-not-zero    TX_AUTH                              (will-inc!    HUB_STAMP    (hub-stamp-inc-for-TX_AUTH-phase))))
(defconstraint    system---hub-stamp-increments---at-the-end-of-TX_INIT                   ()    (if-not-zero    TX_INIT                              (will-inc!    HUB_STAMP    CON)))
(defconstraint    system---hub-stamp-increments---at-the-end-of-TX_FINL                   ()    (if-not-zero    TX_FINL                              (will-inc!    HUB_STAMP    CON)))

;; the HUB_STAMP should increment, in the TX_AUTH phase
;; - with every new authorization tuple
;; - on the final row (where TXN ≡ <true>)
;; - when starting the TX_INIT or TX_SKIP phase that follows
;;
;; These conditions can overlap (TXN is the first perspective in both TX_SKIP and TX_INIT)
;; whence the ` ~ `
(defun   (hub-stamp-inc-for-TX_AUTH-phase)   (~   (next  (+  PEEK_AT_AUTHORIZATION
                                                             PEEK_AT_TRANSACTION
                                                             TX_WARM
                                                             TX_SKIP
                                                             TX_INIT
                                                             ))))

(defconstraint    system---hub-stamp-increments---when-both-counters-are-maxed-out-in-TX_EXEC ()
                  (if-not-zero TX_EXEC
                               (begin
                                 (if-not-zero    (-    CT_TLI    TLI)    (will-remain-constant!    HUB_STAMP))
                                 (if-not-zero    (-    CT_NSR    NSR)    (will-remain-constant!    HUB_STAMP))
                                 (if-eq                CT_TLI    TLI
                                                       (if-eq                CT_NSR    NSR
                                                                             (will-inc!    HUB_STAMP    1))))))

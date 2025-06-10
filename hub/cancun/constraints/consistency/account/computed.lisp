(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;; FIRST/AGAIN/FINAL in conflation ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (acp_FIRST_IN_CNF)
  (fwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         ))
(defcomputed
  (acp_AGAIN_IN_CNF)
  (fwd-unchanged-within    acp_PEEK_AT_ACCOUNT ;; perspective
                           acp_ADDRESS_HI      ;; columns
                           acp_ADDRESS_LO
                           ))
(defcomputed
  (acp_FINAL_IN_CNF)
  (bwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;; FIRST/AGAIN/FINAL in block ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (acp_FIRST_IN_BLK)
  (fwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         acp_REL_BLK_NUM
                         ))
(defcomputed
  (acp_AGAIN_IN_BLK)
  (fwd-unchanged-within    acp_PEEK_AT_ACCOUNT ;; perspective
                           acp_ADDRESS_HI      ;; columns
                           acp_ADDRESS_LO
                           acp_REL_BLK_NUM
                           ))
(defcomputed
  (acp_FINAL_IN_BLK)
  (bwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         acp_REL_BLK_NUM
                         ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;; FIRST/AGAIN/FINAL in transaction ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (acp_FIRST_IN_TXN)
  (fwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         acp_ABS_TX_NUM
                         ))
(defcomputed
  (acp_AGAIN_IN_TXN)
  (fwd-unchanged-within    acp_PEEK_AT_ACCOUNT ;; perspective
                           acp_ADDRESS_HI      ;; columns
                           acp_ADDRESS_LO
                           acp_ABS_TX_NUM
                           ))
(defcomputed
  (acp_FINAL_IN_TXN)
  (bwd-changes-within    acp_PEEK_AT_ACCOUNT ;; perspective
                         acp_ADDRESS_HI      ;; columns
                         acp_ADDRESS_LO
                         acp_ABS_TX_NUM
                         ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                       ;;
;; FRIRST/FINAL deployment number and existence in block ;;
;;                                                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK)
  (fwd-fill-within      acp_PEEK_AT_ACCOUNT
                        acp_FIRST_IN_BLK
                        acp_DEPLOYMENT_NUMBER
                        ))

(defcomputed
  (acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK)
  (bwd-fill-within      acp_PEEK_AT_ACCOUNT
                        acp_FINAL_IN_BLK
                        acp_DEPLOYMENT_NUMBER_NEW
                        ))

(defcomputed
  (acp_EXISTS_FIRST_IN_BLOCK)
  (fwd-fill-within      acp_PEEK_AT_ACCOUNT
                        acp_FIRST_IN_BLK
                        acp_EXISTS
                        ))

(defcomputed
  (acp_EXISTS_FINAL_IN_BLOCK)
  (bwd-fill-within      acp_PEEK_AT_ACCOUNT
                        acp_FINAL_IN_BLK
                        acp_EXISTS_NEW
                        ))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;; Binary constraints ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    account-consistency---binarities ()
                  (begin
                    ( is-binary   acp_FIRST_IN_CNF )     ( is-binary   acp_FIRST_IN_BLK )     ( is-binary   acp_FIRST_IN_TXN )
                    ( is-binary   acp_AGAIN_IN_CNF )     ( is-binary   acp_AGAIN_IN_BLK )     ( is-binary   acp_AGAIN_IN_TXN )
                    ( is-binary   acp_FINAL_IN_CNF )     ( is-binary   acp_FINAL_IN_BLK )     ( is-binary   acp_FINAL_IN_TXN )
                    ))

(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                 ;;
;; FIRST/AGAIN/FINAL in conflation ;;
;;                                 ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (scp_FIRST_IN_CNF)
  (fwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         ))
(defcomputed
  (scp_AGAIN_IN_CNF)
  (fwd-unchanged-within    scp_PEEK_AT_STORAGE ;; perspective
                           scp_ADDRESS_HI      ;; columns
                           scp_ADDRESS_LO
                           scp_STORAGE_KEY_HI
                           scp_STORAGE_KEY_LO
                           ))
(defcomputed
  (scp_FINAL_IN_CNF)
  (bwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;; FIRST/AGAIN/FINAL in block ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (scp_FIRST_IN_BLK)
  (fwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         scp_REL_BLK_NUM
                         ))
(defcomputed
  (scp_AGAIN_IN_BLK)
  (fwd-unchanged-within    scp_PEEK_AT_STORAGE ;; perspective
                           scp_ADDRESS_HI      ;; columns
                           scp_ADDRESS_LO
                           scp_STORAGE_KEY_HI
                           scp_STORAGE_KEY_LO
                           scp_REL_BLK_NUM
                           ))
(defcomputed
  (scp_FINAL_IN_BLK)
  (bwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         scp_REL_BLK_NUM
                         ))

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                  ;;
;; FIRST/AGAIN/FINAL in transaction ;;
;;                                  ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (scp_FIRST_IN_TXN)
  (fwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         scp_ABS_TX_NUM
                         ))
(defcomputed
  (scp_AGAIN_IN_TXN)
  (fwd-unchanged-within    scp_PEEK_AT_STORAGE ;; perspective
                           scp_ADDRESS_HI      ;; columns
                           scp_ADDRESS_LO
                           scp_STORAGE_KEY_HI
                           scp_STORAGE_KEY_LO
                           scp_ABS_TX_NUM
                           ))
(defcomputed
  (scp_FINAL_IN_TXN)
  (bwd-changes-within    scp_PEEK_AT_STORAGE ;; perspective
                         scp_ADDRESS_HI      ;; columns
                         scp_ADDRESS_LO
                         scp_STORAGE_KEY_HI
                         scp_STORAGE_KEY_LO
                         scp_ABS_TX_NUM
                         ))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                         ;;
;; FRIRST/FINAL deployment number in block ;;
;;                                         ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (scp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK)
  (map-if

    ;; target perspective and key columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; target selector
    scp_PEEK_AT_STORAGE
    ;; target key columns
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_REL_BLK_NUM

    ;; source perspective and key/value columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; source selector
    acp_PEEK_AT_ACCOUNT
    ;; source key columns
    acp_ADDRESS_HI
    acp_ADDRESS_LO
    acp_REL_BLK_NUM
    ;; source value column
    acp_DEPLOYMENT_NUMBER_FIRST_IN_BLOCK
    )
  )

(defcomputed
  (scp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK)
  (map-if
    ;; target perspective and key columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; perspective column
    scp_PEEK_AT_STORAGE
    ;; target key columns
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_REL_BLK_NUM

    ;; source perspective and key/value columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; perspective column
    acp_PEEK_AT_ACCOUNT
    ;; source key columns
    acp_ADDRESS_HI
    acp_ADDRESS_LO
    acp_REL_BLK_NUM
    ;; source value column
    acp_DEPLOYMENT_NUMBER_FINAL_IN_BLOCK
    )
  )

(defcomputed
  (scp_EXISTS_FIRST_IN_BLOCK)
  (map-if

    ;; target perspective and key columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; target selector
    scp_PEEK_AT_STORAGE
    ;; target key columns
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_REL_BLK_NUM

    ;; source perspective and key/value columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; source selector
    acp_PEEK_AT_ACCOUNT
    ;; source key columns
    acp_ADDRESS_HI
    acp_ADDRESS_LO
    acp_REL_BLK_NUM
    ;; source value column
    acp_EXISTS_FIRST_IN_BLOCK
    )
  )

(defcomputed
  (scp_EXISTS_FINAL_IN_BLOCK)
  (map-if
    ;; target perspective and key columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; perspective column
    scp_PEEK_AT_STORAGE
    ;; target key columns
    scp_ADDRESS_HI
    scp_ADDRESS_LO
    scp_REL_BLK_NUM

    ;; source perspective and key/value columns
    ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

    ;; perspective column
    acp_PEEK_AT_ACCOUNT
    ;; source key columns
    acp_ADDRESS_HI
    acp_ADDRESS_LO
    acp_REL_BLK_NUM
    ;; source value column
    acp_EXISTS_FINAL_IN_BLOCK
    )
  )


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;; Binary constraints ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    storage-consistency---binarities ()
                  (begin
                    (is-binary   scp_FIRST_IN_CNF )     (is-binary   scp_FIRST_IN_BLK )     (is-binary   scp_FIRST_IN_TXN )
                    (is-binary   scp_AGAIN_IN_CNF )     (is-binary   scp_AGAIN_IN_BLK )     (is-binary   scp_AGAIN_IN_TXN )
                    (is-binary   scp_FINAL_IN_CNF )     (is-binary   scp_FINAL_IN_BLK )     (is-binary   scp_FINAL_IN_TXN )
                    ))

(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                       ;;;;
;;;;    X.5 SELFDESTRUCT   ;;;;
;;;;                       ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                       ;;
;;    X.5.4 Shorthands   ;;
;;                       ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconst
  ROFF_SELFDESTRUCT___STACK_ROW                                           -1
  ROFF_SELFDESTRUCT___SCENARIO_ROW                                         0
  ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW                                      1
  ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW                              2
  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW                              3
  ROFF_SELFDESTRUCT___ACCOUNT___1ST_UNDOING_ROW                            4
  ROFF_SELFDESTRUCT___ACCOUNT___2ND_UNDOING_ROW                            5
  ROFF_SELFDESTRUCT___ACCOUNT_DELETION_ROW                                 4
  ROFF_SELFDESTRUCT___FINAL_CONTEXT_STATICX                                2
  ROFF_SELFDESTRUCT___FINAL_CONTEXT_OOGX                                   4
  ROFF_SELFDESTRUCT___FINAL_CONTEXT_WILL_REVERT                            6
  ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_ALREADY_MARKED             4
  ROFF_SELFDESTRUCT___FINAL_CONTEXT_WONT_REVERT_NOT_YET_MARKED             5
  )

;;
(defun    (selfdestruct-instruction---raw-recipient-address-hi)  (shift [stack/STACK_ITEM_VALUE_HI 1]       ROFF_SELFDESTRUCT___STACK_ROW))   ;; ""
(defun    (selfdestruct-instruction---raw-recipient-address-lo)  (shift [stack/STACK_ITEM_VALUE_LO 1]       ROFF_SELFDESTRUCT___STACK_ROW))   ;; ""
(defun    (selfdestruct-instruction---STATICX)                   (shift stack/STATICX                       ROFF_SELFDESTRUCT___STACK_ROW))
(defun    (selfdestruct-instruction---OOGX)                      (shift stack/OOGX                          ROFF_SELFDESTRUCT___STACK_ROW))
;;
(defun    (selfdestruct-instruction---is-static)                 (shift context/IS_STATIC                   ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW))
(defun    (selfdestruct-instruction---is-deployment)             (shift context/BYTE_CODE_DEPLOYMENT_STATUS ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW))
(defun    (selfdestruct-instruction---account-address-hi)        (shift context/ACCOUNT_ADDRESS_HI          ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW))
(defun    (selfdestruct-instruction---account-address-lo)        (shift context/ACCOUNT_ADDRESS_LO          ROFF_SELFDESTRUCT___1ST_CONTEXT_ROW))
;;
(defun    (selfdestruct-instruction---balance)                   (shift account/BALANCE                     ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
(defun    (selfdestruct-instruction---is-marked)                 (shift account/MARKED_FOR_DELETION         ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))
(defun    (selfdestruct-instruction---had-no-code-initially)     (force-bin (- 1 
                                                                 (shift account/HAD_CODE_INITIALLY          ROFF_SELFDESTRUCT___ACCOUNT___1ST_DOING_ROW))))
;;
(defun    (selfdestruct-instruction---recipient-address-hi)      (shift account/ADDRESS_HI                  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))
(defun    (selfdestruct-instruction---recipient-address-lo)      (shift account/ADDRESS_LO                  ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))
(defun    (selfdestruct-instruction---recipient-trm-flag)        (shift account/TRM_FLAG                    ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))
(defun    (selfdestruct-instruction---recipient-exists)          (shift account/EXISTS                      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))
(defun    (selfdestruct-instruction---recipient-warmth)          (shift account/WARMTH                      ROFF_SELFDESTRUCT___ACCOUNT___2ND_DOING_ROW))

(defun    (selfdestruct-instruction---account-address)           (+ (* (^ 256 LLARGE) (selfdestruct-instruction---account-address-hi))   (selfdestruct-instruction---account-address-lo)))
(defun    (selfdestruct-instruction---recipient-address)         (+ (* (^ 256 LLARGE) (selfdestruct-instruction---recipient-address-hi)) (selfdestruct-instruction---recipient-address-lo)))  ;; ""

(defun    (selfdestruct-instruction---trigger-future-acc-deletion)          
          (* (- 1 (selfdestruct-instruction---is-marked))
             (selfdestruct-instruction---had-no-code-initially)))

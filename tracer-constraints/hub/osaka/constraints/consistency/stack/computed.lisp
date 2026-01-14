(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                        ;;
;; FIRST/AGAIN in context ;;
;;                        ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (stkcp_FIRST_CTXT)
  (fwd-changes-within      stkcp_PEEK_AT_STACK_POW_4 ;; perspective
                           stkcp_CN_POW_4            ;; columns
                           ))
(defcomputed
  (stkcp_AGAIN_CTXT)
  (fwd-unchanged-within      stkcp_PEEK_AT_STACK_POW_4 ;; perspective
                             stkcp_CN_POW_4            ;; columns
                             ))


;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;; FIRST/AGAIN in spot ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (stkcp_FIRST_SPOT)
  (fwd-changes-within      stkcp_PEEK_AT_STACK_POW_4 ;; perspective
                           stkcp_CN_POW_4            ;; columns
                           stkcp_HEIGHT_1234
                           ))
(defcomputed
  (stkcp_AGAIN_SPOT)
  (fwd-unchanged-within      stkcp_PEEK_AT_STACK_POW_4 ;; perspective
                             stkcp_CN_POW_4            ;; columns
                             stkcp_HEIGHT_1234
                             ))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;; Binary constraints ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    stack-consistency---binarities ()
                  (begin
                    ( is-binary   stkcp_FIRST_CTXT )    ( is-binary   stkcp_AGAIN_CTXT )
                    ( is-binary   stkcp_FIRST_SPOT )    ( is-binary   stkcp_AGAIN_SPOT )
                    ))

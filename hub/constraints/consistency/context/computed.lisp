(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;; FIRST/AGAIN for contexts ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defcomputed
  (con_FIRST)
  (fwd-changes-within    ccp_PEEK_AT_CONTEXT ;; perspective
                         ccp_CONTEXT_NUMBER  ;; columns
                         ))

(defcomputed
  (con_AGAIN)
  (fwd-unchanged-within    ccp_PEEK_AT_CONTEXT ;; perspective
                           ccp_CONTEXT_NUMBER  ;; columns
                           ))


;;;;;;;;;;;;;;;;;;;;;;;;
;;                    ;;
;; Binary constraints ;;
;;                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    context-consistency---binarities ()
                  (begin
                    ( is-binary   con_FIRST )
                    ( is-binary   con_AGAIN )
                    ))

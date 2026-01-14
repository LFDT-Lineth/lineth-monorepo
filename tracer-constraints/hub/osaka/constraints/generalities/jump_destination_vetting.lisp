(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                          ;;
;;   4.6 Setting JUMP_DESTINATION_VETTING   ;;
;;                                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint jump-destination-vetting
               (:guard PEEK_AT_STACK)
               ;;;;;;;;;;;;;;;;;;;;;;
               (begin
                 (debug (is-binary stack/JUMP_DESTINATION_VETTING_REQUIRED))
                 (if-zero (force-bin stack/JUMP_FLAG)
                          (vanishes! stack/JUMP_DESTINATION_VETTING_REQUIRED))
                 (if-not-zero (+ stack/SUX stack/SOX stack/OOGX)
                              (vanishes! stack/JUMP_DESTINATION_VETTING_REQUIRED))))

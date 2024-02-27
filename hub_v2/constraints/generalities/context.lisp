(module hub_v2)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;   4.2 Context numbers and context changes   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                               ;;
;;   4.2.1 Setting the CONTEXT_MAY_CHANGE flag   ;;
;;                                               ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


;; (defconstraint setting-the-context-may-change-flag ()
;;                (begin
;;                  (is-binary                             CMC)
;;                  (hub-stamp-constancy                   CMC)
;;                  (if-zero TX_EXEC            (vanishes! CMC))
;;                  (if-not-zero PEEK_AT_STACK
;;                               (eq! (exception_flag_sum) XAHOY))))


;; we subsume parts of XAHOY and CMC under as follows:
;; (cmc_and_xahoy_weighted_sum) = CMC + 2 * XAHOY

(defun (cmc_and_xahoy_weighted_sum) (+ CMC XAHOY XAHOY))
(defun (cmc_sum) (+ XAHOY stack/CALL_FLAG stack/CREATE_FLAG stack/HALT_FLAG))

(defconstraint setting-CMC-and-XAHOY ()
               (begin
                 (is-binary                                                     CMC)
                 (is-binary                                                   XAHOY)
                 (hub-stamp-constancy      (vanishes! (cmc_and_xahoy_weighted_sum)))
                 (if-zero TX_EXEC          (vanishes! (cmc_and_xahoy_weighted_sum)))
                 (if-not-zero PEEK_AT_STACK
                               (begin
                                 (eq! (exception_flag_sum) XAHOY)
                                 (if-zero (cmc_sum)
                                          (vanishes! CMC)
                                          (eq! CMC 1))))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   4.2.2 Consequences of CMC = 1   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint consequences-of-CMC ()
               (if-not-zero CMC
                            (if-not-zero (will-remain-constant! HUB_STAMP)
                                         (begin
                                           (eq! PEEK_AT_CONTEXT 1)))))


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                          ;;
;;   4.2.3 Context number   ;;
;;                          ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defconstraint context-number-generalities (:perspective stack)
               (begin
                 (any! (eq! CN_NEW CN)
                       (eq! CN_NEW CALLER_CN)
                       (eq! CN_NEW (+ 1 HUB_STAMP)))
                 (if-zero CMC
                          (eq! CN_NEW CN))
                 (if-not-zero XAHOY
                              (begin
                                (vanishes! GAS_NEXT)
                                (eq! CN_NEW CALLER_CN)
                                (eq! CN_SELF_REV 1)
                                (eq! CN_REV_STAMP HUB_STAMP)
                                (if-zero CN_NEW (eq! TX_END_STAMP (+ 1 HUB_STAMP)))))
                 (if-zero XAHOY
                          (begin
                            (eq! GAS_NEXT (- GAS_ACTL GAS_COST))
                            ;; TODO: finish
                            ))))

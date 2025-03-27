(module hub)

;;;;;;;;;;;;;;;;;
;;             ;;
;;   4.4 Gas   ;;
;;             ;;
;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                   ;;
;;   4.4.1 Gas column generalities   ;;
;;                                   ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint    gas-columns---stamp-constancies ()
                  (begin (hub-stamp-constancy GAS_EXPECTED)
                         (hub-stamp-constancy GAS_ACTUAL)
                         (hub-stamp-constancy GAS_COST)
                         (hub-stamp-constancy GAS_NEXT)))


;; TODO: should be debug --- rmk: careful analysis should prove that they are indeed redundant; kepping them for now for safety;
(defconstraint    gas-columns---automatic-vanishing ()
                  (if-zero   TX_EXEC
                             (begin
                               (vanishes! GAS_EXPECTED)
                               (vanishes! GAS_ACTUAL)
                               (vanishes! GAS_COST)
                               (vanishes! GAS_NEXT))))

;; we drop the stack perspective preconditions
(defconstraint    gas-columns---GAS_NEXT-vanishes-in-case-of-an-exception ()
                  (if-not-zero   XAHOY
                                 (vanishes!   GAS_NEXT)))

(defconstraint    gas-columns---setting-GAS_NEXT-outside-of-CALLs-and-CREATEs (:perspective stack)
                  (if-zero   XAHOY
                             (if-zero   (force-bin  (+  stack/CREATE_FLAG  stack/CALL_FLAG))
                                        (eq!  GAS_NEXT (- GAS_ACTUAL GAS_COST)))))

(defun    (hub-stamp-transition-within-TX_EXEC)   (*   (will-remain-constant! HUB_STAMP)
                                                       TX_EXEC
                                                       (next TX_EXEC)))

(defconstraint    gas-columns---hub-stamp-transition-constraints---no-context-change ()
                  (if-not-zero (hub-stamp-transition-within-TX_EXEC)
                               (if-eq    CN_NEW    CN
                                         (eq! (next GAS_ACTUAL) (next GAS_EXPECTED)))))

(defconstraint    gas-columns---hub-stamp-transition-constraints---re-entering-parent-context ()
                  (if-not-zero (hub-stamp-transition-within-TX_EXEC)
                               (if-eq    CN_NEW    CALLER_CN
                                         (eq! (next GAS_ACTUAL) (+ (next GAS_EXPECTED) GAS_NEXT)))))

(defconstraint    gas-columns---hub-stamp-transition-constraints---entering-child-context ()
                  (if-not-zero (hub-stamp-transition-within-TX_EXEC)
                               (if-eq    CN_NEW    (+ 1 HUB_STAMP)
                                         (eq! (next GAS_ACTUAL) (next GAS_EXPECTED)))))
;; can't define GAS_EXPECTED at this level of generality

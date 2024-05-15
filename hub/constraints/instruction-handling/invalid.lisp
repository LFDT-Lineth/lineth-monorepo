(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;    X.5.27 Invalid   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;; NOTE: bytes from the invalid instruction family
;; (ither not an opcode or the INVALID opcode)
;; can't raise stack exceptions
(defun (invalid-instruction)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK
     stack/INVALID_FLAG))

(defconstraint invalid-setting-the-stack-pattern (:guard (invalid-instruction))
               (stack-pattern-0-0))

;; already enforced in automatic-exception-flag-vanishing constraint
;; TODO: remove the (vanishes! 0)
(defconstraint invalid-setting-the-OPCX          (:guard (invalid-instruction))
               (begin (vanishes! 0)
                      (debug (eq! stack/OPCX 1))))

;; TODO: remove the (vanishes! 0)
(defconstraint invalid-setting-the-peeking-flags (:guard (invalid-instruction))
               (begin (vanishes! 0)
                      (debug (eq! (next PEEK_AT_CONTEXT) 1))
                      (debug (eq! CMC 1))))

(defconstraint invalid-setting-the-gas-cost      (:guard (invalid-instruction))
               (eq! GAS_COST stack/STATIC_GAS))

(defconstraint invalid-debugging-constraints         (:guard (invalid-instruction))
               (begin
                 (eq! XAHOY CMC)
                 (debug (eq! XAHOY stack/OPCX))))

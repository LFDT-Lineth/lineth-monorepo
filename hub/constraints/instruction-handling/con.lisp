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
;;    X.5.27 Context   ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;; NOTE: bytes from the invalid instruction family
;; (ither not an opcode or the INVALID opcode)
;; can't raise stack exceptions
(defun (context-instruction-no-stack-exception)
  ;;;;;;;;;;;;;;;;;;;;;;;;;;
  (* PEEK_AT_STACK
     stack/CON_FLAG
     (- 1 stack/SUX stack/SOX)))

(defun (decoded-flag-sum)
  (+ [stack/DEC_FLAG 1]
     [stack/DEC_FLAG 2]
     [stack/DEC_FLAG 3]
     [stack/DEC_FLAG 4]))


(defconstraint context-setting-the-stack-pattern (:guard (context-instruction-no-stack-exception))
               (stack-pattern-0-1))

(defconstraint context-setting-the-gas-cost      (:guard (context-instruction-no-stack-exception))
               (eq! GAS_COST stack/STATIC_GAS))

(defconstraint context-setting-NSR               (:guard (context-instruction-no-stack-exception))
               (eq! NSR
                    (+ 1 CMC)))

(defconstraint context-setting-peeking-flags     (:guard (context-instruction-no-stack-exception))
               (begin 
                 (eq! NSR (+ (shift PEEK_AT_CONTEXT 1)
                             (* CMC (shift PEEK_AT_CONTEXT 2))))
                 (read-context-data 1 CONTEXT_NUMBER)))
                      
(defconstraint context-value-constraints         (:guard (context-instruction-no-stack-exception))
               (if-zero CMC
                        (begin
                          (if-not-zero [ stack/DEC_FLAG 1 ]         (begin (eq! [ stack/STACK_ITEM_VALUE_HI 4] context/ACCOUNT_ADDRESS_HI)
                                                                           (eq! [ stack/STACK_ITEM_VALUE_LO 4] context/ACCOUNT_ADDRESS_LO)))
                          (if-not-zero [ stack/DEC_FLAG 2 ]         (begin (eq! [ stack/STACK_ITEM_VALUE_HI 4] context/CALLER_ADDRESS_HI)
                                                                           (eq! [ stack/STACK_ITEM_VALUE_LO 4] context/CALLER_ADDRESS_LO)))
                          (if-not-zero [ stack/DEC_FLAG 3 ]         (begin (eq! [ stack/STACK_ITEM_VALUE_HI 4] 0)
                                                                           (eq! [ stack/STACK_ITEM_VALUE_LO 4] context/CALL_VALUE)))
                          (if-not-zero [ stack/DEC_FLAG 4 ]         (begin (eq! [ stack/STACK_ITEM_VALUE_HI 4] 0)
                                                                           (eq! [ stack/STACK_ITEM_VALUE_LO 4] context/CALL_DATA_SIZE)))
                          (if-zero (force-bin (decoded-flag-sum))   (begin (eq! [ stack/STACK_ITEM_VALUE_HI 4] 0)
                                                                           (eq! [ stack/STACK_ITEM_VALUE_LO 4] context/RETURN_DATA_SIZE))))))

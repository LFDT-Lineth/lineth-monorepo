(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                               ;;;;
;;;;    X.5 Instruction handling   ;;;;
;;;;                               ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                                              ;;
;;    X.5.1 Introduction                                        ;;
;;    X.5.2 Instructions raising the ADD_FLAG                   ;;
;;    X.5.3 Instructions raising the BIN_FLAG                   ;;
;;    X.5.4 Instructions raising the EXT_FLAG                   ;;
;;    X.5.5 Instructions raising the MOD_FLAG                   ;;
;;    X.5.6 Instructions raising the MUL_FLAG                   ;;
;;    X.5.7 Instructions raising the SHF_FLAG                   ;;
;;    X.5.8 Instructions raising the WCP_FLAG                   ;;
;;    X.5.9 Constraints for the preceding insruction families   ;;
;;                                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;  Shorthands  ;;
;;;;;;;;;;;;;;;;;;


(defun (classifier-stateless-instructions) (+ stack/ADD_FLAG
                                              stack/BIN_FLAG
                                              stack/EXT_FLAG
                                              stack/MOD_FLAG
                                              stack/MUL_FLAG
                                              stack/SHF_FLAG
                                              stack/WCP_FLAG ))
(defun (stateless-instruction-is-exp)   (* stack/MUL_FLAG
                                           [ stack/DEC_FLAG 2 ]))
(defun (stateless-instruction-isnt-exp) (+ stack/ADD_FLAG
                                           stack/BIN_FLAG
                                           stack/EXT_FLAG
                                           stack/MOD_FLAG
                                           stack/SHF_FLAG
                                           stack/WCP_FLAG
                                           (* stack/MUL_FLAG [ stack/DEC_FLAG 1 ])))
(defun (one-arg-stateless-instruction) (* (+ stack/BIN_FLAG stack/WCP_FLAG)
                                          [ stack/DEC_FLAG 1 ]))
(defun (two-arg-stateless-instruction) (+ stack/ADD_FLAG 
                                          (* stack/BIN_FLAG (- 1 [ stack/DEC_FLAG 1 ]))
                                          stack/MOD_FLAG
                                          stack/MUL_FLAG
                                          stack/SHF_FLAG
                                          (* stack/WCP_FLAG (- 1 [ stack/DEC_FLAG 1 ]))))
(defun (three-arg-stateless-instruction) stack/EXT_FLAG)

;;  Constraints  ;;
;;;;;;;;;;;;;;;;;;;

(defun (stateless-precondition) (* PEEK_AT_STACK (- 1 stack/SUX stack/SUX)))

;; TODO: comment out
;; sanity check
(defconstraint add-bin-ext-mod-mul-shf-wcp-safeguard (:guard PEEK_AT_STACK)
               (begin
                 (eq! (classifier-stateless-instructions)
                      (+ (stateless-instruction-is-exp)
                         (stateless-instruction-isnt-exp)))
                 (eq! (classifier-stateless-instructions)
                      (+ (one-arg-stateless-instruction)
                         (two-arg-stateless-instruction)
                         (three-arg-stateless-instruction)))))

(defconstraint stateless-stack-pattern (:guard (stateless-precondition))
               (begin
                 (if-not-zero (one-arg-stateless-instruction)   (stack-pattern-1-1))
                 (if-not-zero (two-arg-stateless-instruction)   (stack-pattern-2-1))
                 (if-not-zero (three-arg-stateless-instruction) (stack-pattern-3-1))))

(defconstraint wcp-result-is-binary (:guard (stateless-precondition))
               (if-not-zero stack/WCP_FLAG 
                            (begin 
                              (vanishes! [ stack/STACK_ITEM_VALUE_HI 4 ])
                              (debug (is-binary [ stack/STACK_ITEM_VALUE_LO 4 ])))))

(defconstraint stateless-setting-nsr (:guard (stateless-precondition))
               (if-not-zero (classifier-stateless-instructions)
                            (eq! NON_STACK_ROWS
                                 (+ (stateless-instruction-is-exp) CONTEXT_MAY_CHANGE))))

(defconstraint stateless-setting-peeking-flags (:guard (stateless-precondition))
               (begin
                 (if-not-zero (stateless-instruction-isnt-exp)
                              (eq! NON_STACK_ROWS
                                   (* CMC (next PEEK_AT_CONTEXT))))
                 (if-not-zero (stateless-instruction-is-exp)
                              (eq! NON_STACK_ROWS
                                   (+ (next PEEK_AT_MISCELLANEOUS)
                                      (* CMC (shift PEEK_AT_CONTEXT 2)))))))

(defconstraint stateless-setting-miscellaneous-flags (:guard (stateless-precondition))
               (if-not-zero (classifier-stateless-instructions)
                            (eq! (weighted-MISC-flag-sum 1)
                                 (* (stateless-instruction-is-exp) MISC_WEIGHT_EXP))))

(defconstraint stateless-setting-exp-arguments (:guard (stateless-precondition))
               (if-not-zero (stateless-instruction-is-exp)
                            (set-EXP-instruction-exp-log
                              1                                  ;; row offset
                              [ stack/STACK_ITEM_VALUE_HI 2 ]  ;; exponent high
                              [ stack/STACK_ITEM_VALUE_LO 2 ]  ;; exponent low
                              )))

(defconstraint stateless-gas-cost (:guard (stateless-precondition))
               (begin
                 (if-not-zero (stateless-instruction-isnt-exp)
                              (eq! GAS_COST stack/STATIC_GAS))
                 (if-not-zero (stateless-instruction-is-exp)
                              (eq! GAS_COST
                                   (+ stack/STATIC_GAS
                                      (next [ misc/EXP_DATA 5 ]))))))

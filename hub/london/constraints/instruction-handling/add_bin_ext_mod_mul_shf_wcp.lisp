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


(defun (stateless-instructions---classifier) (+ stack/ADD_FLAG
                                                stack/BIN_FLAG
                                                stack/EXT_FLAG
                                                stack/MOD_FLAG
                                                stack/MUL_FLAG
                                                stack/SHF_FLAG
                                                stack/WCP_FLAG ))
(defun (stateless-instruction---is-EXP)   (* stack/MUL_FLAG
                                             [ stack/DEC_FLAG 2 ]))
(defun (stateless-instruction---isnt-EXP) (+ stack/ADD_FLAG
                                             stack/BIN_FLAG
                                             stack/EXT_FLAG
                                             stack/MOD_FLAG
                                             stack/SHF_FLAG
                                             stack/WCP_FLAG
                                             (* stack/MUL_FLAG [ stack/DEC_FLAG 1 ])))
(defun (stateless-instruction---1-argument-instruction) (* (+ stack/BIN_FLAG stack/WCP_FLAG)
                                                           [ stack/DEC_FLAG 1 ]))
(defun (stateless-instruction---2-argument-instruction) (+ stack/ADD_FLAG
                                                           (* stack/BIN_FLAG (- 1 [ stack/DEC_FLAG 1 ]))
                                                           stack/MOD_FLAG
                                                           stack/MUL_FLAG
                                                           stack/SHF_FLAG
                                                           (* stack/WCP_FLAG (- 1 [ stack/DEC_FLAG 1 ]))))
(defun (stateless-instruction---3-argument-instruction) stack/EXT_FLAG)

;;  Constraints  ;;
;;;;;;;;;;;;;;;;;;;

(defun (stateless-instruction---precondition) (* PEEK_AT_STACK (- 1 stack/SUX stack/SOX)))

;; ;; stupid sanity checks
;; (defconstraint add-bin-ext-mod-mul-shf-wcp-safeguard (:guard PEEK_AT_STACK)
;;                (begin
;;                  (eq! (stateless-instructions---classifier)
;;                       (+ (stateless-instruction---is-EXP)
;;                          (stateless-instruction---isnt-EXP)))
;;                  (eq! (stateless-instructions---classifier)
;;                       (+ (stateless-instruction---1-argument-instruction)
;;                          (stateless-instruction---2-argument-instruction)
;;                          (stateless-instruction---3-argument-instruction)))))

(defconstraint stateless-instruction---stack-pattern (:guard (stateless-instruction---precondition))
               (begin
                 (if-not-zero (stateless-instruction---1-argument-instruction)   (stack-pattern-1-1))
                 (if-not-zero (stateless-instruction---2-argument-instruction)   (stack-pattern-2-1))
                 (if-not-zero (stateless-instruction---3-argument-instruction) (stack-pattern-3-1))))

(defconstraint wcp-result-is-binary (:guard (stateless-instruction---precondition))
               (if-not-zero stack/WCP_FLAG
                            (begin
                              (vanishes! [ stack/STACK_ITEM_VALUE_HI 4 ])
                              (debug (is-binary [ stack/STACK_ITEM_VALUE_LO 4 ])))))

(defconstraint stateless-instruction---setting-nsr (:guard (stateless-instruction---precondition))
               (if-not-zero (stateless-instructions---classifier)
                            (eq! NON_STACK_ROWS
                                 (+ (stateless-instruction---is-EXP) CONTEXT_MAY_CHANGE))))

(defconstraint stateless-instruction---setting-peeking-flags (:guard (stateless-instruction---precondition))
               (begin
                 (if-not-zero (stateless-instruction---isnt-EXP)
                              (eq! NON_STACK_ROWS
                                   (* CMC (next PEEK_AT_CONTEXT))))
                 (if-not-zero (stateless-instruction---is-EXP)
                              (eq! NON_STACK_ROWS
                                   (+ (next PEEK_AT_MISCELLANEOUS)
                                      (* CMC (shift PEEK_AT_CONTEXT 2)))))))

(defconstraint stateless-instruction---setting-miscellaneous-flags (:guard (stateless-instruction---precondition))
               (if-not-zero (stateless-instruction---is-EXP)
                            (eq! (weighted-MISC-flag-sum 1)
                                 MISC_WEIGHT_EXP)))

(defconstraint stateless-instruction---setting-EXP-arguments (:guard (stateless-instruction---precondition))
               (if-not-zero (stateless-instruction---is-EXP)
                            (set-EXP-instruction-exp-log
                              1                                ;; row offset
                              [ stack/STACK_ITEM_VALUE_HI 2 ]  ;; exponent high
                              [ stack/STACK_ITEM_VALUE_LO 2 ]  ;; exponent low
                              )))

(defconstraint stateless-instruction---gas-cost (:guard (stateless-instruction---precondition))
               (begin
                 (if-not-zero (stateless-instruction---isnt-EXP)
                              (eq! GAS_COST stack/STATIC_GAS))
                 (if-not-zero (stateless-instruction---is-EXP)
                              (eq! GAS_COST
                                   (+ stack/STATIC_GAS
                                      (next [ misc/EXP_DATA 5 ]))))))

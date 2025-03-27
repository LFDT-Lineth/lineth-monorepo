(module hub)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;                                ;;;;
;;;;    X.16 Instruction handling   ;;;;
;;;;                                ;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                             ;;
;;    X.16 Instructions raising the BTC_FLAG   ;;
;;                                             ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                              ;;
;;    X.16.1 Supported instructions and flags   ;;
;;    X.16.2 Constraints                        ;;
;;                                              ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defun   (block-data-standard-hypothesis)   (*  PEEK_AT_STACK
                                                stack/BTC_FLAG
                                                (-  1  stack/SUX  stack/SOX)))

;; ;; should be redundant
;; (defconstraint   block-data-instruction-setting-acceptable-exceptions   (:guard (block-data-standard-hypothesis))
;;                  (eq!   XAHOY
;;                         stack/OOGX))

(defconstraint   block-data-instruction-setting-the-stack-pattern       (:guard (block-data-standard-hypothesis))
                 (if-zero   (force-bin   [stack/DEC_FLAG   1])
                            (stack-pattern-0-1)
                            (stack-pattern-1-1)))

(defconstraint   block-data-instruction-setting-NSR                     (:guard (block-data-standard-hypothesis))
                 (eq!   NON_STACK_ROWS
                        CMC))

;; ;; should be redundant
;; (defconstraint   block-data-instruction-setting-the-peeking-flags       (:guard (block-data-standard-hypothesis))
;;                  (if-not-zero   CMC
;;                                 (eq!   (next   PEEK_AT_CONTEXT)
;;                                        1)))

(defconstraint   block-data-instruction-setting-the-gas-cost            (:guard (block-data-standard-hypothesis))
                 (eq!   GAS_COST
                        stack/STATIC_GAS))

(module hub)



(defconstraint   transient-storage-instruction---setting-the-stack-pattern
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (load-store-stack-pattern (transient-storage-instruction---is-TSTORE)))

(defconstraint   transient-storage-instruction---valid-exceptions
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (begin
                   (if-not-zero   (transient-storage-instruction---is-TLOAD)    (eq! XAHOY      stack/OOGX))
                   (if-not-zero   (transient-storage-instruction---is-TSTORE)   (eq! XAHOY   (+ stack/STATICX
                                                                                                stack/OOGX)))))

(defconstraint   transient-storage-instruction---setting-NSR-and-peeking-flags---exceptional-case
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; exceptional case
                 ;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    XAHOY
                                 (begin
                                   (eq! NSR 2)
                                   (eq! NSR
                                        (+ (shift  PEEK_AT_CONTEXT  TRANSIENT___EXCEPTIONAL___CONTEXT_CURRENT___ROFF )
                                           (shift  PEEK_AT_CONTEXT  TRANSIENT___EXCEPTIONAL___CONTEXT_PARENT___ROFF  ))))))

(defun   (tstore-must-be-undone)   (*  (transient-storage-instruction---is-TSTORE)   CN_WILL_REV))
(defun   (transient-no-exception)            (- 1 XAHOY))

(defconstraint   transient-storage-instruction---setting-NSR-and-peeking-flags---unexceptional-case
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 ;; unexceptional case
                 ;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (transient-no-exception)
                                 (begin
                                   (eq! NSR  (+  2  (tstore-must-be-undone)))
                                   (eq! NSR
                                        (+    (shift      PEEK_AT_CONTEXT                              TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF      )
                                              (shift      PEEK_AT_TRANSIENT                            TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF   )
                                           (* (shift      PEEK_AT_TRANSIENT                            TRANSIENT___UNEXCEPTIONAL___TRANSIENT_UNDOING_ROFF )
                                              (tstore-must-be-undone))
                                              )))))

(defconstraint   transient-storage-instruction---first-context-row
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (read-context-data   TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF  ;; row offset
                                      CONTEXT_NUMBER                                 ;; context to read
                                      ))

(defconstraint   transient-storage-instruction---setting-the-STATICX-flag
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (eq!   stack/STATICX
                        (*   (transient-storage-instruction---is-TSTORE)
                             (shift   context/IS_STATIC   TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF))))

(defun (transient-oogx-or-no-exception) (+ stack/OOGX (transient-no-exception)))

(defconstraint   transient-storage-instruction---setting-storage-slot-parameters
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (transient-no-exception)
                                 (begin
                                   (eq! (shift transient/ADDRESS_HI        TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF) (shift context/ACCOUNT_ADDRESS_HI   TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF))
                                   (eq! (shift transient/ADDRESS_LO        TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF) (shift context/ACCOUNT_ADDRESS_LO   TRANSIENT___UNEXCEPTIONAL___CONTEXT_CURR_ROFF))
                                   (eq! (shift transient/STORAGE_KEY_HI    TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF) (transient-storage-instruction---transient-key-hi))
                                   (eq! (shift transient/STORAGE_KEY_LO    TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF) (transient-storage-instruction---transient-key-lo))
                                   (DOM-SUB-stamps---standard              TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF  ;; kappa
                                                                           0))))                                             ;; c

(defconstraint   transient-storage-instruction---setting-storage-slot-values---unexceptional-TLOAD
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (transient-storage-instruction---is-TLOAD)
                                (if-not-zero   (transient-no-exception)
                                               (begin
                                                 (transient-storage-reading            TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)
                                                 (eq!  (shift transient/VALUE_CURR_HI  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)  (transient-storage-instruction---value-to-tload-hi) )
                                                 (eq!  (shift transient/VALUE_CURR_LO  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)  (transient-storage-instruction---value-to-tload-lo) )))))

(defconstraint   transient-storage-instruction---setting-storage-slot-values---unexceptional-TSTORE---doing
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero    (transient-storage-instruction---is-TSTORE)
                                 (if-not-zero   (transient-no-exception)
                                                (begin
                                                  (eq!  (shift transient/VALUE_NEXT_HI  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)  (transient-storage-instruction---value-to-tstore-hi) )
                                                  (eq!  (shift transient/VALUE_NEXT_LO  TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)  (transient-storage-instruction---value-to-tstore-lo) )))))

(defconstraint   transient-storage-instruction---setting-storage-slot-values---unexceptional-TSTORE---undoing
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (transient-storage-instruction---is-TSTORE)
                                (if-not-zero   (transient-no-exception)
                                               (if-not-zero   CONTEXT_WILL_REVERT
                                                              (begin
                                                                (transient-storage-same-slot              TRANSIENT___UNEXCEPTIONAL___TRANSIENT_UNDOING_ROFF   TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)
                                                                (transient-storage-undoing-value-update   TRANSIENT___UNEXCEPTIONAL___TRANSIENT_UNDOING_ROFF   TRANSIENT___UNEXCEPTIONAL___TRANSIENT_DOING_ROFF)
                                                                (DOM-SUB-stamps---revert-with-current     TRANSIENT___UNEXCEPTIONAL___TRANSIENT_UNDOING_ROFF   ;; row offset
                                                                                                          0)
                                                                )))))

(defconstraint   transient-storage-instruction---setting-gas-costs
                 (:guard (transient-storage-instruction---no-stack-exceptions))
                 ;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
                 (if-not-zero   (transient-oogx-or-no-exception)
                                (eq!   GAS_COST
                                       stack/STATIC_GAS)))

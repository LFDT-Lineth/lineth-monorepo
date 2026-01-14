(module rlptxn)

(defconstraint   IS_RLP_PREFIX-phase---byte-size-countdowns-remain-constant-throughout
                 (:guard IS_RLP_PREFIX)
                 (begin
                   (eq! LT_BYTE_SIZE_COUNTDOWN (next LT_BYTE_SIZE_COUNTDOWN))
                   (eq! LX_BYTE_SIZE_COUNTDOWN (next LX_BYTE_SIZE_COUNTDOWN))))

(defproperty     IS_RLP_PREFIX-phase---setting-ct-max
                 (if-not-zero   IS_RLP_PREFIX   (vanishes! CT_MAX)))

(defun    (first-row-of-IS_RLP_PREFIX)    (* IS_RLP_PREFIX TXN))

;; row i + 1
(defconstraint   IS_RLP_PREFIX-phase---first-computation-row---transaction-type-prefix
                 (:guard (first-row-of-IS_RLP_PREFIX))
                 (let   ((ROFF   1))
                   (begin
                     (limb-of-both-lt-and-lx      ROFF)
                     (if-not-zero (shift TYPE_0   ROFF)
                                  (discard-limb   ROFF)
                                  (set-limb       ROFF
                                                  (* txn/TX_TYPE (^ 256 LLARGEMO))
                                                  1)) ;; ""
                     (vanishes! (next PHASE_END))
                     )))

;; row i + 2
(defconstraint   IS_RLP_PREFIX-phase---second-computation-row---global-prefix-for-LT
                 (:guard (first-row-of-IS_RLP_PREFIX))
                 (let   ((ROFF   2))
                   (begin
                     (limb-of-lt-only                                  ROFF)
                     (rlputils-call---BYTE_STRING_PREFIX-non-trivial   ROFF
                                                                       LT_BYTE_SIZE_COUNTDOWN
                                                                       1)
                     (vanishes! (shift PHASE_END                       ROFF))
                     )))

;; row i + 3
(defconstraint   IS_RLP_PREFIX-phase---third-computation-row---global-prefix-for-LX
                 (:guard (first-row-of-IS_RLP_PREFIX))
                 (let   ((ROFF   3))
                   (begin
                     (limb-of-lx-only                                  ROFF)
                     (rlputils-call---BYTE_STRING_PREFIX-non-trivial   ROFF
                                                                       LX_BYTE_SIZE_COUNTDOWN
                                                                       1)
                     (eq! (shift PHASE_END                             ROFF) 1)
                     )))

(module rlptxn)

(defconstraint phase-global-prefix (:guard IS_AUTHORIZATION_LIST)
(if-not-zero (rlptxn---authorization-list---first-CMP-row)
  (rlp-compound-constraint---BYTE_STRING_PREFIX-non-trivial    0
                                                               (rlptxn---authorization-list---list-RLP-length-countdown)
                                                               1
                                                               1)
))

(defconstraint tuple-rlpization (:guard IS_AUTHORIZATION_LIST)
  (if-not-zero (rlptxn---authorization-list---first-row-tuple-processing)
    (begin
    (vanishes! (shift (rlptxn---authorization-list---item-RLP-length-countdown) ( - RLP_TXN_NB_ROWS_PER_AUTHORIZATION 1)))
    ;; prefix
    (rlp-compound-constraint---BYTE_STRING_PREFIX-non-trivial    0
                                                                 (rlptxn---authorization-list---item-RLP-length-countdown)
                                                                 1
                                                                 1)
    ;; chain id
    (rlp-compound-constraint---INTEGER   1
                                         (rlptxn---authorization-list---chain-id-hi)
                                         (rlptxn---authorization-list---chain-id-lo)
                                         0)
    ;; address
    (rlp-compound-constraint---ADDRESS   4
                                         (rlptxn---authorization-list---address-hi)
                                         (rlptxn---authorization-list---address-lo))
    ;; nonce
    (rlp-compound-constraint---INTEGER   7
                                         0
                                         (rlptxn---authorization-list---nonce)
                                         0)
    ;; y
    (rlp-compound-constraint---INTEGER   10
                                         0
                                         (rlptxn---authorization-list---y)
                                         0)
    ;; r
    (rlp-compound-constraint---INTEGER   13
                                         (rlptxn---authorization-list---r-hi)
                                         (rlptxn---authorization-list---r-lo)
                                         0)
    ;; s
    (rlp-compound-constraint---INTEGER   16
                                         (rlptxn---authorization-list---s-hi)
                                         (rlptxn---authorization-list---s-lo)
                                         (rlptxn---authorization-list---end-of-phase))
)))

(defconstraint end-of-auth-phase (:guard IS_AUTHORIZATION_LIST)
(if (== PHASE_END 1)
(vanishes! (rlptxn---authorization-list---list-RLP-length-countdown))
))

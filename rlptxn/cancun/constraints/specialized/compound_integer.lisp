(module rlptxn)

(defun (rlp-compound-constraint---INTEGER   relOffset
                                            integer-hi
                                            integer-lo
                                            is-end-of-phase)
  (begin
    ;; setting CT_MAX
    (eq! (shift CT_MAX   relOffset)   RLP_TXN_CT_MAX_INTEGER)
    ;; constraining PHASE_END
    (eq! (shift PHASE_END  (+ relOffset RLP_TXN_CT_MAX_INTEGER))     is-end-of-phase)
    ;; RLP_UTILS instruction call
    (rlputils-call---INTEGER   relOffset
                               integer-hi
                               integer-lo)
    ;; enshrining the integer's RLP prefix into the RLP string
    (conditionally-set-limb  relOffset
                             (rlptxn---INTEGER---OUT-rlp-prefix-required   relOffset)
                             (rlptxn---INTEGER---OUT-rlp-prefix            relOffset)
                             1)
    ;; enshrining the hi part of a (nonzero) integer into the RLP string
    (conditionally-set-limb   (+ relOffset 1)
                              (rlptxn---INTEGER---OUT-integer-has-non-zero-hi-part   relOffset)
                              (rlptxn---INTEGER---OUT-leading-limb-left-shifted      relOffset)
                              (rlptxn---INTEGER---OUT-leading-limb-byte-size         relOffset))
    ;; enshrining the lo part of a (nonzero) integer into the RLP string
    (conditionally-set-limb   (+ relOffset 2)
                              (rlptxn---INTEGER---OUT-integer-is-non-zero   relOffset)
                              (rlptxn---INTEGER---last-limb integer-lo      relOffset)
                              (rlptxn---INTEGER---last-limb-byte-size       relOffset))
    ))

;; deriving shorthands
(defun (rlptxn---INTEGER---OUT-integer-is-non-zero                relOffset)   (shift  cmp/EXO_DATA_3  relOffset))
(defun (rlptxn---INTEGER---OUT-integer-has-non-zero-hi-part       relOffset)   (shift  cmp/EXO_DATA_4  relOffset))
(defun (rlptxn---INTEGER---OUT-rlp-prefix-required                relOffset)   (shift  cmp/EXO_DATA_5  relOffset))
(defun (rlptxn---INTEGER---OUT-rlp-prefix                         relOffset)   (shift  cmp/EXO_DATA_6  relOffset))
(defun (rlptxn---INTEGER---OUT-leading-limb-left-shifted          relOffset)   (shift  cmp/EXO_DATA_7  relOffset))
(defun (rlptxn---INTEGER---OUT-leading-limb-byte-size             relOffset)   (shift  cmp/EXO_DATA_8  relOffset))
(defun (rlptxn---INTEGER---last-limb     integer-lo               relOffset)
  (if-zero (rlptxn---INTEGER---OUT-integer-has-non-zero-hi-part   relOffset)
           (rlptxn---INTEGER---OUT-leading-limb-left-shifted      relOffset)
           integer-lo))
(defun (rlptxn---INTEGER---last-limb-byte-size relOffset)
  (if-zero (rlptxn---INTEGER---OUT-integer-has-non-zero-hi-part   relOffset)
           (rlptxn---INTEGER---OUT-leading-limb-byte-size         relOffset)
           LLARGE))

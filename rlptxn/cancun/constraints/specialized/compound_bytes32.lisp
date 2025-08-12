(module rlptxn)

(defun (rlp-compound-constraint---BYTES32    relOffset
                                             data-hi
                                             data-lo)
  (begin 
    ;; setting CT_MAX
    (eq! (shift CT_MAX relOffset) RLP_TXN_CT_MAX_BYTES32)
    ;; RLP_UTILS instruction call
    (rlputils-call---BYTES32   relOffset
                               data-hi
                               data-lo)
    ;; enshrining the RLP-prefix into the RLP string
    (set-limb   relOffset
                (* 160 (^ 256 LLARGEMO))
                1) ;; ""
    ;; enshrining data-hi into the RLP string
    (set-limb   (+ relOffset 1)
                data-hi
                LLARGE)
    ;; enshrining data-lo into the RLP string
    (set-limb   (+ relOffset 2)
                data-lo
                LLARGE)
    ))

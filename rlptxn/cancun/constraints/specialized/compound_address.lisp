(module rlptxn)

(defun (rlp-compound-constraint---ADDRESS    relOffset
                                             address-hi
                                             address-lo)
  (begin 
    ;; setting CT_MAX
    (eq! (shift CT_MAX            relOffset)      RLP_TXN_CT_MAX_ADDRESS)
    ;; calling RLPADDR
    (eq! (shift cmp/TRM_FLAG      relOffset)      1)
    (eq! (shift cmp/EXO_DATA_1    relOffset)      address-hi)
    (eq! (shift cmp/EXO_DATA_2    relOffset)      address-lo)
    ;; enshrining the RLP-prefix into the RLP string
    (set-limb    relOffset
                 (* 148 (^ 256 LLARGEMO)) ;; "" ... ""
                 1)
    ;; enshrining the hi part of the address into the RLP string
    (set-limb    (+ relOffset 1)
                 (* address-hi (^ 256 12))
                 4)
    ;; enshrining the hi part of the address into the RLP string
    (set-limb    (+ relOffset 2)
                 address-lo
                 LLARGE)
    ))

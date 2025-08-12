(module rlputils)

;; hook
(defun (integer-instruction-precondition) (* MACRO IS_INTEGER))

;; shorthands

(defun (integer--in-integer-hi)                                     macro/DATA_1)
(defun (integer--in-integer-lo)                                     macro/DATA_2)
(defun (integer--out-integer-is-nonzero)                            macro/DATA_3)
(defun (integer--out-integer-has-nonzero-hi-part)                   macro/DATA_4)
(defun (integer--out-rlp-prefix-required)                           macro/DATA_5)
(defun (integer--out-rlp-prefix)                                    macro/DATA_6)
(defun (integer--out-leading-limb-left-shifted)                     macro/DATA_7)
(defun (integer--out-leading-limb-byte-size)                        macro/DATA_8)

;; constraints
(defconstraint integer--setting-ct-max (:guard (integer-instruction-precondition)) 
    (eq! (next CT_MAX) CT_MAX_INST_INTEGER))

;; first row
(defconstraint integer--first-wcp-call   (:guard (integer-instruction-precondition))  (wcp-call-iszero  1 (integer--in-integer-hi) (integer--in-integer-lo)))

(defun (integer--integer-is-zero)                                   (shift compt/RES 1))
(defun (integer--integer-is-nonzero)                                (force-bin (- 1 (integer--integer-is-zero))))

;;second row
(defconstraint integer--second-wcp-call  (:guard (integer-instruction-precondition))  (wcp-call-gt      2  0               (integer--in-integer-hi) 0))

(defun (integer--integer-hi-is-nonzero)                             (shift compt/RES 2))
(defun (integer--integer-hi-is-zero)                                (force-bin (- 1 (integer--integer-hi-is-nonzero))))
(defun (integer--integer-hi-byte-size)                              (+ (shift compt/WCP_CT_MAX 2) (integer--integer-hi-is-nonzero)))

;; third row
(defconstraint integer--third-wcp-call   (:guard (integer-instruction-precondition))  (wcp-call-lt      3  0               (integer--in-integer-lo) RLP_PREFIX_INT_SHORT))

(defun (integer--integer-is-lt-one-two-eight)                       (* (integer--integer-hi-is-zero) (shift compt/RES 3)))
(defun (integer--integer-is-geq-one-two-eight)                      (force-bin (- 1 (integer--integer-is-lt-one-two-eight))))
(defun (integer--integer-lo-byte-size)                              (+ (shift compt/WCP_CT_MAX 3) (integer--integer-is-nonzero)))
(defun (integer--leading-limb-byte-size)                            (+ (* (integer--integer-hi-is-nonzero) (integer--integer-hi-byte-size))
                                                                       (* (integer--integer-hi-is-zero)    (integer--integer-lo-byte-size))))
(defun (integer--integer-byte-size)                                 (+ (integer--leading-limb-byte-size) (* (integer--integer-hi-is-nonzero) LLARGE)))

;; setting results
(defconstraint integer--setting-shift-factor (:guard (integer-instruction-precondition)) 
    (conditionally-get-shifting-factor 3 (integer--integer-is-nonzero) (integer--leading-limb-byte-size)))

(defun (integer--shifting-factor)                                   (shift compt/SHF_POWER 3))
(defun (integer--leading-limb)                                      (+ (* (integer--integer-hi-is-nonzero) (integer--in-integer-hi))
                                                                       (* (integer--integer-hi-is-zero)    (integer--in-integer-lo))))
(defun (integer--leading-limb-left-shifted)                         (* (integer--shifting-factor) (integer--leading-limb)))

(defconstraint integer--setting-result (:guard (integer-instruction-precondition))
    (begin 
    (eq! (integer--out-integer-is-nonzero)               (integer--integer-is-nonzero))
    (eq! (integer--out-integer-has-nonzero-hi-part)      (integer--integer-hi-is-nonzero))
    (eq! (integer--out-rlp-prefix-required)              (+ (integer--integer-is-zero) (integer--integer-is-geq-one-two-eight)))    
    (eq! (integer--out-rlp-prefix)                       (* (integer--out-rlp-prefix-required) (+ RLP_PREFIX_INT_SHORT (integer--integer-byte-size)) (^ 256 LLARGEMO)))
    (eq! (integer--out-leading-limb-left-shifted)        (integer--leading-limb-left-shifted))
    (eq! (integer--out-leading-limb-byte-size)           (integer--leading-limb-byte-size))
    ))  
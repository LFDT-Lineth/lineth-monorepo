(module rlputils)

;; hook
(defun (bytestring--instruction-precondition) (* MACRO IS_BYTE_STRING_PREFIX))

;; shorthands

(defun (bytestring--in-byte-string-length)                             macro/DATA_1)
(defun (bytestring--in-byte-string-first-byte)                         macro/DATA_2)
(defun (bytestring--in-byte-string-is-list)                            macro/DATA_3)
(defun (bytestring--out-byte-string-is-nonempty)                       macro/DATA_4)
(defun (bytestring--out-rlp-prefix-required)                           macro/DATA_5)
(defun (bytestring--out-rlp-prefix)                                    macro/DATA_6)
(defun (bytestring--out-rlp-prefix-byte-size)                          macro/DATA_8)

(defun (base-rlp-prefix-short)                             (+ (* RLP_PREFIX_INT_SHORT  (- 1 (bytestring--in-byte-string-is-list)))
                                                              (* RLP_PREFIX_LIST_SHORT      (bytestring--in-byte-string-is-list))))
(defun (base-rlp-prefix-long)                              (+ (* RLP_PREFIX_INT_LONG   (- 1 (bytestring--in-byte-string-is-list)))
                                                              (* RLP_PREFIX_LIST_LONG       (bytestring--in-byte-string-is-list))))

;; constraints
(defconstraint bytestring--setting-ct-max (:guard (bytestring--instruction-precondition)) 
    (eq! (next CT_MAX) CT_MAX_INST_BYTE_STRING_PREFIX))

;; first row
(defconstraint bytestring--first-wcp-call   (:guard (bytestring--instruction-precondition))  
    (wcp-call-iszero 1 0 (bytestring--in-byte-string-length)))

(defun (bytestring--bs-is-empty)                                       (shift compt/RES 1))
(defun (bytestring--bs-is-nonempty)                                    (force-bin (- 1 (bytestring--bs-is-empty))))

(defconstraint bytestring--justifying-empty-byte-string   (:guard (bytestring--instruction-precondition)) 
    (eq! (bytestring--out-byte-string-is-nonempty) (bytestring--bs-is-nonempty)))

;; justifying empty byte string result

(defconstraint bytestring--justifying-output-empty-byte-string   (:guard (bytestring--instruction-precondition)) 
    (if-not-zero (bytestring--bs-is-empty)
    (begin
         (eq! (bytestring--out-rlp-prefix-required)  1 )
         (eq! (bytestring--out-rlp-prefix)           (* (base-rlp-prefix-short) (^ 256 LLARGEMO)))
         (eq! (bytestring--out-rlp-prefix-byte-size) 1 ))))

;; second row
(defconstraint bytestring--second-wcp-call   (:guard (bytestring--instruction-precondition))  
    (wcp-call-eq     2  (bytestring--in-byte-string-length)          1))

(defun (bytestring--bsl-is-eq-one)                                     (force-bin (* (bytestring--bs-is-nonempty) (shift compt/RES 2))))
(defun (bytestring--bsl-is-gt-one)                                     (force-bin (* (bytestring--bs-is-nonempty) (- 1 (shift compt/RES 2)))))

;; third row

;; case bsl == 1

(defconstraint bytestring--third-wcp-call-case-bsl-is-one   (:guard (* (bytestring--instruction-precondition) (bytestring--bsl-is-eq-one)))  
    (wcp-call-lt     3 0 (bytestring--in-byte-string-first-byte)        RLP_PREFIX_INT_SHORT))

(defun (bytestring--bsfb-is-lt-one-two-eight)                          (shift compt/RES 3))

(defconstraint bytestring--justifying-output-bsl-is-one   (:guard (bytestring--instruction-precondition)) 
    (if-not-zero (bytestring--bsl-is-eq-one)
        (if-not-zero (bytestring--bsfb-is-lt-one-two-eight)
        ;; case (bytestring--bsfb-is-lt-one-two-eight) == 1
            (eq! (bytestring--out-rlp-prefix-required)  0 )
        ;; case (bytestring--bsfb-is-lt-one-two-eight) == 0
            (begin 
            (eq! (bytestring--out-rlp-prefix-required)  1 )
            (eq! (bytestring--out-rlp-prefix)           (* (+ (base-rlp-prefix-short) 1) (^ 256 LLARGEMO)))
            (eq! (bytestring--out-rlp-prefix-byte-size) 1 )))))

;; case bsl > 1
(defconstraint bytestring--third-wcp-call-case-bsl-is-gt-one   (:guard (* (bytestring--instruction-precondition) (bytestring--bsl-is-gt-one)))  
    (wcp-call-lt     3 0 (bytestring--in-byte-string-length)        56))

(defun (bytestring--bsl-is-lt-fifty-six)                           (shift compt/RES 3))
(defun (bytestring--bsl-byte-size)                                 (+ (shift compt/WCP_CT_MAX 3) 1))

(defconstraint bytestring--justifying-output-bsl-is-gt-one   (:guard (bytestring--instruction-precondition)) 
    (if-not-zero (bytestring--bsl-is-gt-one)
        (if-not-zero (bytestring--bsl-is-lt-fifty-six)
        ;; case (bytestring--bsl-is-lt-fifty-six) == 1
            (begin 
            (eq! (bytestring--out-rlp-prefix-required)  1 )
            (eq! (bytestring--out-rlp-prefix)           (* (+ (base-rlp-prefix-short) (bytestring--in-byte-string-length)) (^ 256 LLARGEMO)))
            (eq! (bytestring--out-rlp-prefix-byte-size) 1 ))

        ;; case (bytestring--bsl-is-lt-fifty-six) == 0
            (begin 
            (get-shifting-factor 3           (+ 1 (bytestring--bsl-byte-size)))
            (eq! (bytestring--out-rlp-prefix-required)  1 )
            (eq! (bytestring--out-rlp-prefix)           (+ (* (+ (base-rlp-prefix-long) (bytestring--bsl-byte-size)) (^ 256 LLARGEMO)) 
                                                           (bytestring--shifted-byte-size)))
            (eq! (bytestring--out-rlp-prefix-byte-size) (+ 1 (bytestring--bsl-byte-size)))))))

(defun (bytestring--shifted-byte-size)                           (* (bytestring--in-byte-string-length) (shift compt/SHF_POWER 3)))
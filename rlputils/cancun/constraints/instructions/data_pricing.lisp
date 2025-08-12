(module rlputils)

;; hook
(defun (data-pricing--instruction-precondition-macro-row) (* MACRO IS_DATA_PRICING))
(defun (data-pricing--instruction-precondition-compt-row) (* COMPT IS_DATA_PRICING))

;; shorthands

(defun (data-pricing--in-limb)                                           macro/DATA_1)
(defun (data-pricing--in-limb-byte-size)                                 macro/DATA_2)
(defun (data-pricing--out-number-of-zero-bytes-in-limb)                  macro/DATA_6)
(defun (data-pricing--out-number-of-nonz-bytes-in-limb)                  macro/DATA_7)
(defun (data-pricing--out-limb-first-byte)                               macro/DATA_8)

(defun (data-pricing--ct-max)                                            (- (data-pricing--in-limb-byte-size) 1))

;; macro hook
(defconstraint data-pricing--initialization (:guard (data-pricing--instruction-precondition-macro-row))
    (begin
    (eq! (next CT_MAX)                                          (data-pricing--ct-max))
    (eq! (next compt/LIMB)                                      (data-pricing--in-limb))
    (eq! (data-pricing--out-number-of-zero-bytes-in-limb)       ZERO_COUNTER)
    (eq! (data-pricing--out-number-of-nonz-bytes-in-limb)       NONZ_COUNTER)
    (eq! (data-pricing--out-limb-first-byte)                    (next compt/ARG_1_LO))))

(defproperty sum-of-zero-and-nonz-sanity-check 
    (if-not-zero (data-pricing--instruction-precondition-macro-row) (eq! (data-pricing--in-limb-byte-size) (+ (data-pricing--out-number-of-zero-bytes-in-limb) 
                                                                                                              (data-pricing--out-number-of-nonz-bytes-in-limb)))))

;; compt rows

(defun (data-pricing--is-zero-byte)                                      
    (force-bin (if-zero compt/ARG_1_LO 
                1 
                0)))
(defun (data-pricing--is-nonz-byte)     (force-bin (- 1 (data-pricing--is-zero-byte))))

(defconstraint  data-pricing--compt-rows (:guard (data-pricing--instruction-precondition-compt-row))
    (begin 
    ;; byte decomposition:
    (if-zero CT 
        (eq! compt/ACC compt/ARG_1_LO)
        (eq! compt/ACC (+ (* 256 (prev compt/ACC))
                          compt/ARG_1_LO)))
    (wcp-call-leq     0 0 compt/ARG_1_LO 255)
    (result-must-be-true 0)
    ;; decrementing the counter
    (eq! ZERO_COUNTER (- (prev ZERO_COUNTER) (data-pricing--is-zero-byte)))
    (eq! NONZ_COUNTER (- (prev NONZ_COUNTER) (data-pricing--is-nonz-byte)))
    ;; finalization constraint
    (if (== CT CT_MAX)
        (begin
        (vanishes! ZERO_COUNTER)
        (vanishes! NONZ_COUNTER)
        (get-shifting-factor 0  (+ 1 CT_MAX))
        (eq! compt/LIMB (* compt/ACC compt/SHF_POWER))))))
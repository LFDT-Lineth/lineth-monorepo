(module rlp)

(defconst 
  initial-out2-shift (^ 256 9)
  short-list         0xc0  ;; RLP prefix for a short list
  long-number        0x80) ;; RLP prefix for a number > 127

;; shift `x` `b` bytes to the left
(defpurefun (byte-shift x b)
  (* x (^ 256 b)))

;; returns a if nonce == 0; b if 0 < nonce < 128; c else
(defun (cond-nonce a b c)
  (if-zero NONCE
           a
           (if-zero (small-nonce)
                    b
                    c)))

;; the nonce is <=127 iff tn[0] == 0 and there is only
;; one byte in the nonce
(defun (small-nonce)
  (+ (shift tn -7) (- NONCE_n 1)))

;; returns 0 if we are within the same RLP computation
(defun (same-instance)
  (all! (remained-constant! ADDR_HI) (remained-constant! ADDR_LO) (remained-constant! NONCE)))

;;
;; ADDR_LO accumulation
;;
(defconstraint addr-lo-accumulator ()
  (if-zero addr_lo_ndl
           ;; ax builds addr_lo_1
           (if-zero ct
                    (eq! addr_lo_1 addr_lo_ax)
                    (eq! addr_lo_1
                         (+ (* 256 (prev addr_lo_1))
                            addr_lo_ax)))
           ;; ax builds addr_lo_2
           (if-zero (- ct 10)
                    (eq! addr_lo_2 addr_lo_ax)
                    (eq! addr_lo_2
                         (+ (* 256 (prev addr_lo_2))
                            addr_lo_ax)))))

;;
;; Counter cycle & stamp
;;
(defconstraint constancies ()
  (begin (counter-constancy ct ADDR_HI)
         (counter-constancy ct ADDR_LO)
         (counter-constancy ct NONCE)
         (counter-constancy ct STAMP)
         (counter-constancy ct N_BYTES)))

(defconstraint initial-stamp (:domain {0})
  (vanishes! STAMP))

(defconstraint stamp-update ()
  (any! (will-remain-constant! STAMP) (will-inc! STAMP 1)))

(defconstraint zero-counter ()
  (if-zero STAMP
           (vanishes! ct)))

(defconstraint counter-reset ()
  (if-not-zero (will-remain-constant! STAMP)
               (vanishes! (next ct))))

(defconstraint counter-cycle (:guard STAMP)
  (if-eq-else ct 15 (will-inc! STAMP 1) (will-inc! ct 1)))

;;
;; Needle cycle
;;
(defconstraint needle-start ()
  (if-zero ct
           (vanishes! addr_lo_ndl)))

(defconstraint needle-stable-regime ()
  (if-zero (same-instance)
           (any! (remained-constant! addr_lo_ndl) (did-inc! addr_lo_ndl 1))))

(defconstraint needle-switch-regime (:guard ct)
  (if-eq ct 10 (did-inc! addr_lo_ndl 1)))

;;
;; Nonce byte-counting
;;
(defconstraint nonce-recomposition ()
  (if-zero ct
           (eq! NONCE_bytes 0)
           (eq! NONCE_bytes
                (+ (byte-shift (prev NONCE_bytes) 1)
                   NONCE_ax))))

;;
;; OUT/2 shift
;;
(defconstraint out2-shift-start (:guard STAMP)
  (if-zero ct
           (eq! out2_shift initial-out2-shift)))

(defconstraint out2-shift-decrease (:guard STAMP)
  (if-zero (next NONCE_bytes)
           (eq! (next out2_shift) initial-out2-shift)
           (eq! (byte-shift (next out2_shift) 1)
                out2_shift)))

;; when ct = 1, then in-nonce is zero, because the
;; largest possible byte count in the nonce is 8
;; (nonce are 64 bits integers)
(defconstraint in-nonce-start ()
  (if-zero ct
           (vanishes! in-nonce)))

;; in-nonce can't go back within a cycle
(defconstraint in-nonce-monotonous ()
  (if-zero (remained-constant! STAMP)
           (any! (remained-constant! in-nonce) (did-inc! in-nonce 1))
           (vanishes! in-nonce)))

;; in-nonce must light up at the first non-null byte
(defconstraint in-nonce ()
  (if-not-zero NONCE_ax
               (eq! in-nonce 1)))

;; the nonce byte counter must start counting
;; at the first non-0 NONCE_ax
(defconstraint nonce-bytes-start-counting ()
  (if-not-zero NONCE_ax
               (did-inc! NONCE_n 1)))

;; until the last iteration, the nonce byte counter
;; must increase once triggered
(defconstraint nonce-bytes-monotonous ()
  (if-zero ct
           (vanishes! in-nonce)
           (eq! NONCE_n
                (+ (prev NONCE_n) in-nonce))))

;; zero-nonce still occupies one byte
(defconstraint byte-count-zerononce ()
  (if-eq ct 15
         (if-zero NONCE
                  (eq! NONCE_n 1))))

;; validate the bit-decomposition of the nonce LSB
(defconstraint tn-bit-decomposition ()
  (if-eq ct 15
         (eq! NONCE_ax
              (reduce +
                      (for i
                           [0:7]
                           (* (shift tn (neg i))
                              (^ 2 i)))))))

;;
;; Final assembly
;;
(defconstraint byte-count ()
  (if-eq ct 15
         (eq! N_BYTES
              (+ 1
                 1
                 20
                 (cond-nonce 1 1 (+ 1 NONCE_n))))))

(defconstraint check-out-1 ()
  (if-eq ct 15
         (let ((l1_pos 15)
               (l2_pos 14)
               (addr_hi_shift 10)
               (addr_lo_1_shift 0))
              (eq! (prev OUT)
                   (+ (byte-shift (+ short-list                      ;; short list encoding of final result
                                     (+ 1 20)                        ;; encoded address length
                                     (cond-nonce 1 1 (+ 1 NONCE_n))) ;; encoded nonce length
                                  l1_pos)
                      (byte-shift (+ long-number 20) l2_pos)         ;; short list encoding for address
                      (byte-shift ADDR_HI addr_hi_shift)
                      (byte-shift addr_lo_1 addr_lo_1_shift))))))

(defconstraint check-out-2 ()
  (if-eq ct 15
         (let ((addr_lo_2_pos 10)
               (nonce_pos 9))
              (eq! OUT
                   (+ (byte-shift addr_lo_2 addr_lo_2_pos)
                      (cond-nonce (byte-shift long-number nonce_pos)
                                  (byte-shift NONCE nonce_pos)
                                  (+ (byte-shift (+ 0x80 NONCE_n) nonce_pos)
                                     (* NONCE_bytes out2_shift))))))))



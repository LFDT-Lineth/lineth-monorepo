(module rlptxn)

(defun (conditionally-set-limb   w
                                 condition-bit
                                 limb
                                 number-of-bytes)
     (begin
     (eq! (shift LC            w) condition-bit)
     (eq! (shift cmp/LIMB      w) (* condition-bit limb))
     (eq! (shift cmp/LIMB_SIZE w) (* condition-bit number-of-bytes))))

(defun               (set-limb w
                               limb
                               number-of-bytes)
     (begin
     (eq! (shift LC            w) 1)
     (eq! (shift cmp/LIMB      w) limb)
     (eq! (shift cmp/LIMB_SIZE w) number-of-bytes)))

(defun           (discard-limb w)
     (eq! (shift LC            w) 0))


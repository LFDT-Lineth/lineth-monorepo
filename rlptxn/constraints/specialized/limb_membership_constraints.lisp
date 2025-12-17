(module rlptxn)

(defun (limb-of-lt-only        w)
     (begin
     (eq! (shift LT            w) 1)
     (eq! (shift LX            w) 0)))

(defun (limb-of-lx-only        w)
     (begin
     (eq! (shift LT            w) 0)
     (eq! (shift LX            w) 1)))

(defun (limb-of-both-lt-and-lx w)
     (begin
     (eq! (shift LT            w) 1)
     (eq! (shift LX            w) 1)))

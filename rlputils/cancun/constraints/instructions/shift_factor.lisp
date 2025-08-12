(module rlputils)

(defun (conditionally-get-shifting-factor offset condition bytes-away)
    (begin 
    (eq! (shift compt/SHF_FLAG offset) condition)
    (eq! (shift compt/SHF_ARG  offset) (* condition (- LLARGE bytes-away)))))

(defun               (get-shifting-factor offset           bytes-away)
    (begin 
    (eq! (shift compt/SHF_FLAG offset) 1)
    (eq! (shift compt/SHF_ARG  offset) (- LLARGE bytes-away))))
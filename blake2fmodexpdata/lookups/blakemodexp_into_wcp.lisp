(defun (selector)
  (force-bool (* (~ blake2fmodexpdata.STAMP)
                 (- blake2fmodexpdata.STAMP (prev blake2fmodexpdata.STAMP)))))

(deflookup 
  blakemodexp-into-wcp
  ; target colums (in WCP)
  (
    wcp.ARG_1_HI
    wcp.ARG_1_LO
    wcp.ARG_2_HI
    wcp.ARG_2_LO
    wcp.RES
    wcp.INST
  )
  ; source columns
  (
    0
    (* (selector) (prev blake2fmodexpdata.ID))
    0
    (* (selector) blake2fmodexpdata.ID)
    (* (selector) 1)
    (* (selector) EVM_INST_LT)
  ))



(defun (shift-activation-flag) (and hub.SHIFT_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception

(deflookup hub-into-shf
    ;reference columns
    (
        shf.ARG_1_HI
        shf.ARG_1_LO
        shf.ARG_2_HI
        shf.ARG_2_LO
        shf.RES_HI
        shf.RES_LO
        shf.INST
    )
    ;source columns
    (
        (* hub.VAL_HI_1     (shift-activation-flag))
        (* hub.VAL_LO_1     (shift-activation-flag))
        (* hub.VAL_HI_3     (shift-activation-flag))
        (* hub.VAL_LO_3     (shift-activation-flag))
        (* hub.VAL_HI_4     (shift-activation-flag))
        (* hub.VAL_LO_4     (shift-activation-flag))
        (* hub.INSTRUCTION  (shift-activation-flag))
    )
)

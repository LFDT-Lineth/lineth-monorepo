(defun (binary-activation-flag) (and hub.BINARY_INST (- 1 hub.STACK_UNDERFLOW_EXCEPTION))) ;; TODO: gas exception

(deflookup hub-into-bin
    ;reference columns
    (
        bin.ARG_1_HI
        bin.ARG_1_LO
        bin.ARG_2_HI
        bin.ARG_2_LO
        bin.RES_HI
        bin.RES_LO
        bin.INST
    )
    ;source columns
    (
        (* hub.VAL_HI_1     (binary-activation-flag))
        (* hub.VAL_LO_1     (binary-activation-flag))
        (* hub.VAL_HI_3     (binary-activation-flag))
        (* hub.VAL_LO_3     (binary-activation-flag))
        (* hub.VAL_HI_4     (binary-activation-flag))
        (* hub.VAL_LO_4     (binary-activation-flag))
        (* hub.INSTRUCTION  (binary-activation-flag))
    )
)

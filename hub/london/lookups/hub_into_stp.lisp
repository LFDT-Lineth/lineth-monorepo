(defun (hub-into-stp-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS hub.misc/STP_FLAG))

(deflookup
  hub-into-stp
  ;; target columns
  (
    stp.INSTRUCTION
    stp.GAS_HI
    stp.GAS_LO
    stp.VAL_HI
    stp.VAL_LO
    stp.EXISTS
    stp.WARM
    stp.OUT_OF_GAS_EXCEPTION
    stp.GAS_ACTUAL
    stp.GAS_UPFRONT
    stp.GAS_MXP
    stp.GAS_OUT_OF_POCKET
    stp.GAS_STIPEND
  )
  ;; source columns
  (
    (* hub.misc/STP_INSTRUCTION            (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_HI                 (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_LO                 (hub-into-stp-trigger))
    (* hub.misc/STP_VALUE_HI               (hub-into-stp-trigger))
    (* hub.misc/STP_VALUE_LO               (hub-into-stp-trigger))
    (* hub.misc/STP_EXISTS                 (hub-into-stp-trigger))
    (* hub.misc/STP_WARMTH                 (hub-into-stp-trigger))
    (* hub.misc/STP_OOGX                   (hub-into-stp-trigger))
    (* hub.GAS_ACTUAL                      (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_UPFRONT_GAS_COST   (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_MXP                (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_PAID_OUT_OF_POCKET (hub-into-stp-trigger))
    (* hub.misc/STP_GAS_STIPEND            (hub-into-stp-trigger))
  )
  )



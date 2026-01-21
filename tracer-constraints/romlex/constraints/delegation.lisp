(module romlex)

(defconstraint deployment-cant-delegate ()
  (if-not-zero DEPLOYMENT_STATUS (vanishes! COULD_BE_DELEGATION_CODE)))

(defconstraint delegated-have-specific-size ()
  (if-zero  DEPLOYMENT_STATUS
    (if-eq-else CODE_SIZE EIP_7702_DELEGATED_ACCOUNT_CODE_SIZE (eq! COULD_BE_DELEGATION_CODE 1)
                                                               (eq! COULD_BE_DELEGATION_CODE 0))))

(defconstraint not-delegated-is-not-delegated ()
  (if-zero COULD_BE_DELEGATION_CODE
    (begin
      (vanishes! LEADING_THREE_BYTES)
      (vanishes! LEAD_DELEGATION_BYTES)
      (vanishes! TAIL_DELEGATION_BYTES))))

(defconstraint delegate-code-leading-bytes-marker ()
    (if-eq-else LEADING_THREE_BYTES EIP_7702_DELEGATION_INDICATOR  (eq! ACTUALLY_DELEGATION_CODE 1)
                                                                   (eq! ACTUALLY_DELEGATION_CODE 0)))

(defconstraint copy-delegation-address ()
  (begin
    (eq! DELEGATION_ADDRESS_HI (* ACTUALLY_DELEGATION_CODE LEAD_DELEGATION_BYTES))
    (eq! DELEGATION_ADDRESS_LO (* ACTUALLY_DELEGATION_CODE TAIL_DELEGATION_BYTES))))

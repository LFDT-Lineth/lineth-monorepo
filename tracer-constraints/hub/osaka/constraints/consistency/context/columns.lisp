(module hub)

;; ccp_ ⇔ context consistency permutation
(defpermutation
    ;; permuted columns
    ;;;;;;;;;;;;;;;;;;;
  (
      ccp_PEEK_AT_CONTEXT
      ccp_CONTEXT_NUMBER
      ccp_HUB_STAMP
      ccp_UPDATE
      ccp_CALL_STACK_DEPTH
      ccp_IS_ROOT
      ccp_IS_STATIC
      ccp_ACCOUNT_ADDRESS_HI
      ccp_ACCOUNT_ADDRESS_LO
      ccp_ACCOUNT_DEPLOYMENT_NUMBER
      ccp_BYTE_CODE_ADDRESS_HI
      ccp_BYTE_CODE_ADDRESS_LO
      ccp_BYTE_CODE_DEPLOYMENT_NUMBER
      ccp_BYTE_CODE_DEPLOYMENT_STATUS
      ccp_BYTE_CODE_CODE_FRAGMENT_INDEX
      ccp_CALL_DATA_CONTEXT_NUMBER
      ccp_CALLER_ADDRESS_HI
      ccp_CALLER_ADDRESS_LO
      ccp_CALL_VALUE
      ccp_CALL_DATA_OFFSET
      ccp_CALL_DATA_SIZE
      ccp_RETURN_AT_OFFSET
      ccp_RETURN_AT_CAPACITY
      ccp_RETURN_DATA_OFFSET
      ccp_RETURN_DATA_SIZE
      ccp_RETURN_DATA_CONTEXT_NUMBER
  )
  ;; original columns
  ;;;;;;;;;;;;;;;;;;;
  (
    (+ PEEK_AT_CONTEXT)
    (+ context/CONTEXT_NUMBER)
    (+ HUB_STAMP)
    (+ context/UPDATE)
    context/CALL_STACK_DEPTH
    context/IS_ROOT
    context/IS_STATIC
    context/ACCOUNT_ADDRESS_HI
    context/ACCOUNT_ADDRESS_LO
    context/ACCOUNT_DEPLOYMENT_NUMBER
    context/BYTE_CODE_ADDRESS_HI
    context/BYTE_CODE_ADDRESS_LO
    context/BYTE_CODE_DEPLOYMENT_NUMBER
    context/BYTE_CODE_DEPLOYMENT_STATUS
    context/BYTE_CODE_CODE_FRAGMENT_INDEX
    context/CALL_DATA_CONTEXT_NUMBER
    context/CALLER_ADDRESS_HI
    context/CALLER_ADDRESS_LO
    context/CALL_VALUE
    context/CALL_DATA_OFFSET
    context/CALL_DATA_SIZE
    context/RETURN_AT_OFFSET
    context/RETURN_AT_CAPACITY
    context/RETURN_DATA_OFFSET
    context/RETURN_DATA_SIZE
    context/RETURN_DATA_CONTEXT_NUMBER
  )
)

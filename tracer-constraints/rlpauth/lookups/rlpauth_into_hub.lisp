(defclookup
    (rlp-auth-into-hub :unchecked)
    ;; target selector
    (* hub.TX_AUTH hub.PEEK_AT_AUTHORIZATION)
    ;; target columns
    (
        hub.USER_TXN_NUMBER
        hub.HUB_STAMP
        hub.auth/TUPLE_INDEX
        hub.auth/AUTHORITY_ECRECOVER_SUCCESS
        (:: hub.auth/AUTHORITY_ADDRESS_HI hub.auth/AUTHORITY_ADDRESS_LO)
        hub.auth/AUTHORITY_NONCE
        hub.auth/AUTHORITY_HAS_EMPTY_CODE_OR_IS_DELEGATED
        (:: hub.auth/DELEGATION_ADDRESS_HI hub.auth/DELEGATION_ADDRESS_LO)
        hub.auth/DELEGATION_ADDRESS_IS_ZERO
        hub.auth/AUTHORIZATION_TUPLE_IS_VALID
        hub.auth/SENDER_IS_AUTHORITY
        hub.auth/SENDER_IS_AUTHORITY_ACC
    )
    rlpauth.rlpauth_into_hub_lookup_selector
    ;; source columns
    (
        rlpauth.user_txn_number
        rlpauth.hub_stamp
        rlpauth.tuple_index                              ;; justified in HUB
        rlpauth.authority_ecrecover_success              ;; justified in RLPAUTH
        rlpauth.authority_address                        ;; justified in RLPAUTH
        rlpauth.authority_nonce                          ;; justified in the HUB
        rlpauth.authority_has_empty_code_or_is_delegated ;; justified in the HUB
        rlpauth.delegation_address                       ;; justified in RLPAUTH
        rlpauth.delegation_address_is_zero               ;; justified in RLPAUTH
        rlpauth.authorization_tuple_is_valid             ;; justified in RLPAUTH
        rlpauth.sender_is_authority                      ;; justified in RLPAUTH
        rlpauth.sender_is_authority_acc                  ;; justified in the HUB
    ))

;; filter for hub
;; TX_AUTH|PEEK_AT_AUTHORIZATION|USER_TXN_NUMBER|HUB_STAMP|auth/TUPLE_INDEX|auth/AUTHORITY_ECRECOVER_SUCCESS|auth/AUTHORITY_ADDRESS|auth/AUTHORITY_NONCE|auth/AUTHORITY_HAS_EMPTY_CODE_OR_IS_DELEGATED|auth/DELEGATION_ADDRESS|auth/DELEGATION_ADDRESS_IS_ZERO|auth/AUTHORIZATION_TUPLE_IS_VALID|auth/SENDER_IS_AUTHORITY

;; filter for rlp auth
;; rlpauth_into_hub_lookup_selector|user_txn_number|hub_stamp|tuple_index|authority_ecrecover_success|authority_address|authority_nonce|authority_has_empty_code_or_is_delegated|delegation_address|delegation_address_is_zero|authorization_tuple_is_valid|sender_is_authority                               
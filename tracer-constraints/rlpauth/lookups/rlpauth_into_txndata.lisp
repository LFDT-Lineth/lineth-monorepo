(defclookup
    (rlp-auth-into-txndata :unchecked)
    ;; target selector
    (* txndata.USER txndata.HUB (next txndata.rlp/TYPE_4))
    ;; target columns
    (
        txndata.BLK_NUMBER
        txndata.USER_TXN_NUMBER
        (:: txndata.hub/FROM_ADDRESS_HI txndata.hub/FROM_ADDRESS_LO)
    )
    rlpauth.not_padding_selector
    ;; source columns
    (
        rlpauth.blk_number
        rlpauth.user_txn_number
        rlpauth.txn_from_address
    ))

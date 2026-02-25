(defclookup
    (rlp-auth-into-blockdata :unchecked)
    ;; target selector
    blockdata.IS_CHAINID
    ;; target columns
    (
        blockdata.REL_BLOCK
        (:: blockdata.DATA_HI blockdata.DATA_LO)
    )
    rlpauth.not_padding_selector
    ;; source columns
    (
        rlpauth.blk_number
        rlpauth.network_chain_id
    ))
(defclookup
    (rlp-auth-into-shakiradata-3-limbs :unchecked)
    ;; target selector
    (* shakiradata.IS_KECCAK_DATA (~ (- shakiradata.ID (prev shakiradata.ID))))
    ;; target columns
    (
        shakiradata.ID
        shakiradata.LIMB ;; data
        (next shakiradata.LIMB)
        (shift shakiradata.LIMB 2)
        (shift shakiradata.LIMB 3) ;; result
        (shift shakiradata.LIMB 4)
        shakiradata.TOTAL_SIZE
        shakiradata.IS_KECCAK_DATA
        shakiradata.IS_KECCAK_RESULT
        shakiradata.INDEX
        (next shakiradata.INDEX)
        (shift shakiradata.INDEX 2)
        (shift shakiradata.INDEX 3)
        (shift shakiradata.INDEX 4)
    )
    ;; source selector
    keccak.limbs_are_3
    ;; source columns
    (
        keccak.id
        keccak.limb ;; data
        (next keccak.limb)
        (shift keccak.limb 2)
        (shift keccak.limb 3) ;; result
        (shift keccak.limb 4)
        keccak.total_size
        keccak.is_keccak_data
        keccak.is_keccak_result
        keccak.index
        (next keccak.index)
        (shift keccak.index 2)
        (shift keccak.index 3)
        (shift keccak.index 4)
    ))

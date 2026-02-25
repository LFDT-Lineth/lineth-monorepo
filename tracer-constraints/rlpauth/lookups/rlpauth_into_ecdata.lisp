(defclookup
    (rlp-auth-into-ecdata :unchecked)
    ;; target selector
    (* ecdata.IS_ECRECOVER_DATA (~ (- ecdata.ID (prev ecdata.ID)))) ;; alternatively (* ecdata.IS_ECRECOVER_DATA (- 1 (prev ecdata.IS_ECRECOVER_DATA)))
    ;; target columns
    (
        ecdata.ID
        ecdata.LIMB ;; data
        (next ecdata.LIMB)
        (shift ecdata.LIMB 2)
        (shift ecdata.LIMB 3)
        (shift ecdata.LIMB 4)
        (shift ecdata.LIMB 5)
        (shift ecdata.LIMB 6)
        (shift ecdata.LIMB 7)
        (shift ecdata.LIMB 8) ;; result
        (shift ecdata.LIMB 9)
        ecdata.TOTAL_SIZE
        ecdata.SUCCESS_BIT
        ecdata.IS_ECRECOVER_DATA
        ecdata.IS_ECRECOVER_RESULT
        ecdata.INDEX
        (next ecdata.INDEX)
        (shift ecdata.INDEX 2)
        (shift ecdata.INDEX 3)
        (shift ecdata.INDEX 4)
        (shift ecdata.INDEX 5)
        (shift ecdata.INDEX 6)
        (shift ecdata.INDEX 7)
        (shift ecdata.INDEX 8)
        (shift ecdata.INDEX 9)  
    )
    ;; source selector
    ecrecover.lookup_selector
    ;; source columns
    (
        ecrecover.id
        ecrecover.limb ;; data
        (next ecrecover.limb)
        (shift ecrecover.limb 2)
        (shift ecrecover.limb 3)
        (shift ecrecover.limb 4)
        (shift ecrecover.limb 5)
        (shift ecrecover.limb 6)
        (shift ecrecover.limb 7)
        (shift ecrecover.limb 8) ;; result
        (shift ecrecover.limb 9)
        ecrecover.total_size
        ecrecover.success_bit
        ecrecover.is_ecrecover_data
        ecrecover.is_ecrecover_result
        ecrecover.index
        (next ecrecover.index)
        (shift ecrecover.index 2)
        (shift ecrecover.index 3)
        (shift ecrecover.index 4)
        (shift ecrecover.index 5)
        (shift ecrecover.index 6)
        (shift ecrecover.index 7)
        (shift ecrecover.index 8)
        (shift ecrecover.index 9)
    ))

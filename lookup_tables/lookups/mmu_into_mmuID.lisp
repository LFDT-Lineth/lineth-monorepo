(deflookup mmu-into-mmuID
    ;reference columns
    (
        mmuID.INST
        mmuID.INFO
        mmuID.PRE
        mmuID.IS_IN_ID
    )
    ;source columns 
    (
        mmu.INST
        mmu.INFO
        mmu.PRE
        mmu.IS_DATA ; TODO is this required?
    )
)
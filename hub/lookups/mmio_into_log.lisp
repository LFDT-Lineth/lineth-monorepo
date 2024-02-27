(deflookup lookup-mmio-into-log
    ;; source columns
    (
        (* mmio.LOG_NUM mmio.EXO_IS_LOG)
        (* mmio.INDEX_X mmio.EXO_IS_LOG)
        (* mmio.VAL_X mmio.EXO_IS_LOG)
    )

    ;target columns
    (
        log_data.NUM
        log_data.INDEX 
        log_data.LIMB
    )
)

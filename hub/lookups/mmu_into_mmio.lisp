(deflookup lookup-mmu-into-mmio
    ;; source columns 
    (
        (* mmu.MICRO_STAMP mmu.IS_MICRO)
        (* mmu.MICRO_INST mmu.IS_MICRO)
        (* mmu.SBO mmu.IS_MICRO)
        (* mmu.SLO mmu.IS_MICRO)
        (* mmu.TBO mmu.IS_MICRO)
        (* mmu.TLO mmu.IS_MICRO)
        (* mmu.ERF mmu.IS_MICRO)
        (* mmu.FAST mmu.IS_MICRO)
        (* mmu.SIZE mmu.IS_MICRO))
    ;target columns
    (
        mmio.MICRO_STAMP
        mmio.MICRO_INST
        mmio.SBO
        mmio.SLO
        mmio.TBO
        mmio.TLO
        mmio.ERF
        mmio.FAST
        mmio.SIZE))

;; (deflookup lookup-mmio-into-rom
;;                 ()
;;                 ())
;; ; data: (address_hi, address_lo, deployment_number, limb_index, datalimb) <- we don't have the dep# as of
;; (deflookup lookup-mmio-into-log
;;                 ()
;;                 ())
;; ; data: (logNum, limb_index, datalimb) <- logNum grows by 1 with every LOG0, LOG1, LOG2, LOG3, LOG4
;; (deflookup lookup-mmio-into-hash
;;                 ()
;;                 ())
;; ; data: (hashNum, limb_index, datalimb) <- hashNum grows by 1 with every SHA3 and CREATE2
;; (deflookup lookup-mmio-into-txcd
;;                 ()
;;                 ())
;; ; data: (txnum, limb_index, datalimb)

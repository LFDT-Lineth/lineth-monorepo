(defun (op-flag-sum)
  (+ mmio.IS_LIMB_VANISHES
     mmio.IS_LIMB_TO_RAM_TRANSPLANT
     mmio.IS_LIMB_TO_RAM_ONE_TARGET
     mmio.IS_LIMB_TO_RAM_TWO_TARGET
     mmio.IS_RAM_TO_LIMB_TRANSPLANT
     mmio.IS_RAM_TO_LIMB_ONE_SOURCE
     mmio.IS_RAM_TO_LIMB_TWO_SOURCE
     mmio.IS_RAM_TO_RAM_TRANSPLANT
     mmio.IS_RAM_TO_RAM_PARTIAL
     mmio.IS_RAM_TO_RAM_TWO_TARGET
     mmio.IS_RAM_TO_RAM_TWO_SOURCE
     mmio.IS_RAM_EXCISION
     mmio.IS_RAM_VANISHES))

(deflookup
  (mmio-into-mmu :unchecked)
  ;reference columns
  (
    mmu.MICRO
    mmu.MMIO_STAMP
    mmu.micro/INST
    mmu.micro/SIZE
    mmu.micro/SLO
    mmu.micro/SBO
    mmu.micro/TLO
    mmu.micro/TBO
    mmu.micro/LIMB
    mmu.micro/CN_S
    mmu.micro/CN_T
    mmu.micro/SUCCESS_BIT
    mmu.micro/EXO_SUM
    mmu.micro/PHASE
    mmu.micro/EXO_ID
    mmu.micro/KEC_ID
    mmu.micro/TOTAL_SIZE
  )
  ;source columns
  (
    (op-flag-sum)
    mmio.MMIO_STAMP
    mmio.MMIO_INSTRUCTION
    mmio.SIZE
    mmio.SLO
    mmio.SBO
    mmio.TLO
    mmio.TBO
    mmio.LIMB
    mmio.CNS
    mmio.CNT
    mmio.SUCCESS_BIT
    mmio.EXO_SUM
    mmio.PHASE
    mmio.EXO_ID
    mmio.KEC_ID
    mmio.TOTAL_SIZE
  ))



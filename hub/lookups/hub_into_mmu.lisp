(defun (hub-into-mmu-trigger)
  (* hub.PEEK_AT_MISCELLANEOUS hub.misc/MMU_FLAG))

(deflookup
  hub-into-mmu
  ;; target columns
  (
    mmu.MACRO
    mmu.STAMP
    mmu.macro/INST
    mmu.macro/SRC_ID
    mmu.macro/TGT_ID
    mmu.macro/AUX_ID
    mmu.macro/SRC_OFFSET_LO
    mmu.macro/SRC_OFFSET_HI
    mmu.macro/TGT_OFFSET_LO
    mmu.macro/SIZE
    mmu.macro/REF_OFFSET
    mmu.macro/REF_SIZE
    mmu.macro/SUCCESS_BIT
    mmu.macro/LIMB_1
    mmu.macro/LIMB_2
    mmu.macro/PHASE
    mmu.macro/EXO_SUM
  )
  ;; source columns
  (
    (hub-into-mmu-trigger)
    (* hub.MMU_STAMP (hub-into-mmu-trigger))
    (* hub.misc/MMU_INST (hub-into-mmu-trigger))
    (* hub.misc/MMU_SRC_ID (hub-into-mmu-trigger))
    (* hub.misc/MMU_TGT_ID (hub-into-mmu-trigger))
    (* hub.misc/MMU_AUX_ID (hub-into-mmu-trigger))
    (* hub.misc/MMU_SRC_OFFSET_LO (hub-into-mmu-trigger))
    (* hub.misc/MMU_SRC_OFFSET_HI (hub-into-mmu-trigger))
    (* hub.misc/MMU_TGT_OFFSET_LO (hub-into-mmu-trigger))
    (* hub.misc/MMU_SIZE (hub-into-mmu-trigger))
    (* hub.misc/MMU_REF_OFFSET (hub-into-mmu-trigger))
    (* hub.misc/MMU_REF_SIZE (hub-into-mmu-trigger))
    (* hub.misc/MMU_SUCCESS_BIT (hub-into-mmu-trigger))
    (* hub.misc/MMU_LIMB_1 (hub-into-mmu-trigger))
    (* hub.misc/MMU_LIMB_2 (hub-into-mmu-trigger))
    (* hub.misc/MMU_PHASE (hub-into-mmu-trigger))
    (* hub.misc/MMU_EXO_SUM (hub-into-mmu-trigger))
  ))



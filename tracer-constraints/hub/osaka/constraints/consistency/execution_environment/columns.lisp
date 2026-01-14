(module hub)


;; envcp_ ⇔ execution environment consistency permutation
(defpermutation
  ;; permuted columns
  (
   envcp_CN
   envcp_HUB_STAMP
   envcp_CFI
   envcp_CALLER_CN
   envcp_CN_WILL_REV
   envcp_CN_GETS_REV
   envcp_CN_SELF_REV
   envcp_CN_REV_STAMP
   envcp_PC
   envcp_PC_NEW
   envcp_HEIGHT
   envcp_HEIGHT_NEW
   envcp_GAS_EXPECTED
   envcp_GAS_NEXT
   )
  ;; original columns
  (
   (↓ CN )
   (↓ HUB_STAMP )
   CFI
   CALLER_CN
   CN_WILL_REV
   CN_GETS_REV
   CN_SELF_REV
   CN_REV_STAMP
   PC
   PC_NEW
   HEIGHT
   HEIGHT_NEW
   GAS_EXPECTED
   GAS_NEXT
   )
  )

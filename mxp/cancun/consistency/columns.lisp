(module mxp)

(defpermutation
  ;; permuted columns
  ;; mscp ≡ MXP state consistency permutation
  (
   mscp_SCENARIO
   mscp_CN
   mscp_MXP_STAMP
   mscp_C_MEM
   mscp_C_MEM_NEW
   mscp_WORDS
   mscp_WORDS_NEW
   )
  ;;
  ;; inputs
  (
   (+ SCENARIO)
   (+ CN)
   (+ MXP_STAMP)
   scenario/C_MEM
   scenario/C_MEM_NEW
   scenario/WORDS
   scenario/WORDS_NEW
   )
  )

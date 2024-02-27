(module hub_v2)

(defperspective misc
		;; selector
		PEEK_AT_MISCELLANEOUS

		;; miscellaneous-row columns
		(
		 (EXP_FLAG                          :binary@prove)
		 (MMU_FLAG                          :binary@prove)
		 (MXP_FLAG                          :binary@prove)
		 (OOB_FLAG                          :binary@prove)
		 (STP_FLAG                          :binary@prove)

		 ;; EXP columns (DONE)
		 (EXP_INST                :i32)
		 (EXP_DATA                :array [5])

		 ;; MMU columns (DONE)
		 (MMU_INST                :i32)
		 (MMU_SRC_ID              :i32)
		 (MMU_TGT_ID              :i32)
		 (MMU_AUX_ID              :i32)
		 (MMU_SRC_OFFSET_LO       :i128)
		 (MMU_SRC_OFFSET_HI       :i128)
		 (MMU_TGT_OFFSET_LO       :i128)
		 (MMU_SIZE                :i32)
		 (MMU_REF_OFFSET          :i32)
		 (MMU_REF_SIZE            :i32)
		 (MMU_SUCCESS_BIT         :binary@prove)
		 (MMU_LIMB_1              :i128)
		 (MMU_LIMB_2              :i128)
		 (MMU_PHASE               :i32)
		 (MMU_EXO_SUM             :i32)

		 ;; MXP colummns (DONE)
		 MXP_INST
		 (MXP_MXPX                :binary@prove) ;;  ;; TODO: demote to debug constraint, though truly useless
		 (MXP_DEPLOYS             :binary@prove) ;;  ;; TODO: demote to debug constraint
		 MXP_OFFSET_1_HI
		 MXP_OFFSET_1_LO
		 MXP_OFFSET_2_HI
		 MXP_OFFSET_2_LO
		 MXP_SIZE_1_HI
		 MXP_SIZE_1_LO
		 MXP_SIZE_2_HI
		 MXP_SIZE_2_LO
		 MXP_WORDS
		 MXP_GAS_MXP

		 ;; OOB columns (DONE)
		 (OOB_INST                :i32)
		 (OOB_DATA                :array[1:8])

		 ;; STP columns (DONE)
		 STP_INST
		 STP_GAS_HI
		 STP_GAS_LO
		 STP_VAL_HI
		 STP_VAL_LO
		 (STP_EXISTS                        :binary@prove) ;; TODO: demote to debug constraint
		 (STP_WARMTH                        :binary@prove) ;; TODO: demote to debug constraint
		 (STP_OOGX                          :binary@prove) ;; TODO: demote to debug constraint
		 STP_GAS_UPFRONT_GAS_COST
		 STP_GAS_PAID_OUT_OF_POCKET
		 STP_GAS_STIPEND

		 ;; ``truly'' miscellaneous columns
		 (CCSR_FLAG                           :binary@prove)  ;; Child Context Self Reverts Flag; ;; TODO: demote to debug constraint
		 CCRS_STAMP                                           ;; Child Context Revert Stamp
		 ))

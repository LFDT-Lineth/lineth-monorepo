(module hub)

(defperspective misc
		;; selector
		PEEK_AT_MISCELLANEOUS

		;; miscellaneous-row columns
		(
		 ( EXP_FLAG   :binary@prove )
		 ( MMU_FLAG   :binary@prove )
		 ( MXP_FLAG   :binary@prove )
		 ( OOB_FLAG   :binary@prove )
		 ( STP_FLAG   :binary@prove )

		 ;; EXP columns (DONE)
		 ( EXP_INST                :i32 )
		 ( EXP_DATA                :array [5] :i128 )   ;; ""

		 ;; MMU columns (DONE)
		 ( MMU_INST                :i32   :display :hex)
		 ( MMU_SRC_ID              :i32   )
		 ( MMU_TGT_ID              :i32   )
		 ( MMU_AUX_ID              :i32   )
		 ( MMU_SRC_OFFSET_LO       :i128  )
		 ( MMU_SRC_OFFSET_HI       :i128  )
		 ( MMU_TGT_OFFSET_LO       :i128  )
		 ( MMU_SIZE                :i32   )
		 ( MMU_REF_OFFSET          :i32   )
		 ( MMU_REF_SIZE            :i32   )
		 ( MMU_SUCCESS_BIT         :binary@prove )
		 ( MMU_LIMB_1              :i128  )
		 ( MMU_LIMB_2              :i128  )
		 ( MMU_PHASE               :i32   )
		 ( MMU_EXO_SUM             :i32   )

		 ;; MXP colummns      (DONE )
		 ( MXP_INST                     :i32   )
		 ( MXP_MXPX                     :binary@prove ) ;;  ;; TODO: demote to debug constraint, though truly useless
		 ( MXP_DEPLOYS                  :binary@prove ) ;;  ;; TODO: demote to debug constraint
		 ( MXP_OFFSET_1_HI              :i128 )
		 ( MXP_OFFSET_1_LO              :i128 )
		 ( MXP_OFFSET_2_HI              :i128 )
		 ( MXP_OFFSET_2_LO              :i128 )
		 ( MXP_SIZE_1_HI                :i128 )
		 ( MXP_SIZE_1_LO                :i128 )
		 ( MXP_SIZE_2_HI                :i128 )
		 ( MXP_SIZE_2_LO                :i128 )
		 ( MXP_WORDS                    :i128 )
		 ( MXP_GAS_MXP                  :i128 )
		 ( MXP_MTNTOP                   :binary@prove )
		 ( MXP_SIZE_1_NONZERO_NO_MXPX   :binary@prove )
		 ( MXP_SIZE_2_NONZERO_NO_MXPX   :binary@prove )

		 ;; OOB columns (DONE)
		 (OOB_INST                 :i32  )
		 (OOB_DATA                 :array[1:9] :i128 ) ;; ""

		 ;; STP columns (DONE)
		 ( STP_INSTRUCTION               :i32  )   ;; TODO: overkill
		 ( STP_GAS_HI                    :i128 )
		 ( STP_GAS_LO                    :i128 )
		 ( STP_VALUE_HI                  :i128 )
		 ( STP_VALUE_LO                  :i128 )
		 ( STP_EXISTS                    :binary@prove) ;; TODO: demote to debug constraint
		 ( STP_WARMTH                    :binary@prove) ;; TODO: demote to debug constraint
		 ( STP_OOGX                      :binary@prove) ;; TODO: demote to debug constraint
		 ( STP_GAS_MXP                   :i64 )
		 ( STP_GAS_UPFRONT_GAS_COST      :i64 )
		 ( STP_GAS_PAID_OUT_OF_POCKET    :i64 )
		 ( STP_GAS_STIPEND               :i32 )   ;; TODO: in all applications either 0 or 2_300 ... so i12 should suffice

		 ;; ``truly'' miscellaneous columns
		 ( CCSR_FLAG                           :binary@prove)  ;; Child Context Self Reverts Flag; ;; TODO: demote to debug constraint
		 ( CCRS_STAMP                          :i32 )          ;; Child Context Revert Stamp
		 ))

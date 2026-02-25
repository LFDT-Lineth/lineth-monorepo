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
		 ( EXP_INST                :i16 )
		 ( EXP_DATA                :array [5] :i128 ) ;;""

		 ;; MMU columns (DONE)
		 ( MMU_INST                :i16   :display :hex)
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

		 ;; MXP colummns
		 ( MXP_INST                     :byte   )
		 ( MXP_MXPX                     :binary )
		 ( MXP_DEPLOYS                  :binary )
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
		 ( MXP_SIZE_1_NONZERO_NO_MXPX   :binary@prove )
		 ( MXP_SIZE_2_NONZERO_NO_MXPX   :binary@prove )

		 ;; OOB columns
		 (OOB_INST                 :i16  )
		 (OOB_DATA                 :array[1:10] :i128 ) ;;""

		 ;; STP columns
		 ( STP_INSTRUCTION                  :byte   )
		 ( STP_GAS_HI                       :i128   )
		 ( STP_GAS_LO                       :i128   )
		 ( STP_VALUE_HI                     :i128   )
		 ( STP_VALUE_LO                     :i128   )
		 ( STP_EXISTS                       :binary )
		 ( STP_WARMTH                       :binary )
		 ( STP_OOGX                         :binary )
		 ( STP_GAS_MXP                      :i64    )
		 ( STP_GAS_UPFRONT_GAS_COST         :i64    )
		 ( STP_GAS_PAID_OUT_OF_POCKET       :i64    )
		 ( STP_GAS_STIPEND                  :i12    )
		 ( STP_CALLEE_IS_DELEGATED          :binary )
		 ( STP_CALLEE_IS_DELEGATED_TO_SELF  :binary )
		 ( STP_DELEGATE_WARMTH              :binary )


		 ;; ``truly'' miscellaneous columns
		 ( CCSR_FLAG                           :binary)  ;; Child Context Self Reverts Flag;
		 ( CCRS_STAMP                          :i32 )    ;; Child Context Revert Stamp
		 ))

(module mxp)

(defperspective scenario
		;; selector
		SCENARIO
		;; scenario columns
		(
		 ;; scenario flags
		 ( MSIZE                     :binary@prove )
		 ( TRIVIAL                   :binary@prove )
		 ( MXPX                      :binary@prove )
		 ( STATE_UPDATE_WORD_PRICING :binary@prove )
		 ( STATE_UPDATE_BYTE_PRICING :binary@prove )
		 ;; state columns
		 ( WORDS                     :i33          ) ;; i32 + i32 = i33 ...
		 ( WORDS_NEW                 :i33          ) ;; i32 + i32 = i33 ...
		 ( C_MEM                     :i64          )
		 ( C_MEM_NEW                 :i64          )
		 )
		)

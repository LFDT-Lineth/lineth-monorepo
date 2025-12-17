(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                            ;;
;;    X.Y.Y Common columns    ;;
;;                            ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defcolumns
  ;; HUB block and transaction labels + transaction bit markers
  ;; ( BLK_NUMBER      :i24          ) ;; defcomputed
  ;; ( TOTL_TXN_NUMBER :i24          ) ;; defcomputed
  ( SYSI_TXN_NUMBER :i24          )
  ( USER_TXN_NUMBER :i24          )
  ( SYSF_TXN_NUMBER :i24          )
  ( SYSI            :binary       ) ;; binarity is inherited from the HUB
  ( USER            :binary       ) ;; same
  ( SYSF            :binary       ) ;; same
  ;; perspective flags
  ( CMPTN           :binary@prove )
  ( HUB             :binary@prove )
  ( RLP             :binary@prove )
  ;; heartbeat
  ( CT              :i5           )
  ( CT_MAX          :i5           )
  ( GAS_CUMULATIVE  :i64          )
  )


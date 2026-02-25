(module txndata)

;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                ;;
;;    X.Y.Z Binary constraints    ;;
;;                                ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;


(defproperty    binarity-constraints
                (begin

                  ;; shared columns extracted from the HUB
                  ( is-binary   SYSI )
                  ( is-binary   USER )
                  ( is-binary   SYSF )

                  ;; perspective flags
                  ;; ( is-binary   CMPTN  ) ;; is :binary@prove
                  ;; ( is-binary   HUB    )
                  ;; ( is-binary   RLP    )

                  ;; rlp/ columns
                  ;; ( is-binary   rlp/TYPE_0 ) ;; is :binary@prove
                  ;; ( is-binary   rlp/TYPE_1 )
                  ;; ( is-binary   rlp/TYPE_2 )
                  ;; ( is-binary   rlp/TYPE_3 )
                  ;; ( is-binary   rlp/TYPE_4 )

                  ;; hub/ columns
                  ;; ( is-binary hub/IS_DEPLOYMENT              ) ;; is :binary@prove
                  ;; ( is-binary hub/HAS_EIP_1559_GAS_SEMANTICS )
                  ;; ( is-binary hub/REQUIRES_EVM_EXECUTION     )
                  ;; ( is-binary hub/COPY_TXCD                  )
                  ;; ( is-binary hub/STATUS_CODE                )
                  ;; ( is-binary hub/EIP_4788                   )
                  ;; ( is-binary hub/EIP_2935                   )
                  ;; ( is-binary hub/NOOP                       )

                  ;; computation/ columns
                  ;; ( is-binary cmptn/EUC_FLAG )
                  ;; ( is-binary cmptn/WCP_FLAG )
                  ))

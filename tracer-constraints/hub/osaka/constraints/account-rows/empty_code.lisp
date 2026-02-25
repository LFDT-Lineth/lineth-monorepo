(module hub)


;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
;;                                    ;;
;;   X.2 Code ownership constraints   ;;
;;                                    ;;
;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;

(defconstraint   account---code-ownership-curr (:perspective account)
                 (if-eq-else account/CODE_HASH_HI EMPTY_KECCAK_HI
                             (if-eq-else account/CODE_HASH_LO EMPTY_KECCAK_LO
                                         (eq! account/HAS_CODE 0)
                                         (eq! account/HAS_CODE 1))
                             (eq! account/HAS_CODE 1)))

(defconstraint   account---code-ownership-next (:perspective account)
                 (if-eq-else account/CODE_HASH_HI_NEW EMPTY_KECCAK_HI
                             (if-eq-else account/CODE_HASH_LO_NEW EMPTY_KECCAK_LO
                                         (eq! account/HAS_CODE_NEW 0)
                                         (eq! account/HAS_CODE_NEW 1))
                             (eq! account/HAS_CODE_NEW 1)))


(defconstraint   account---precompiles-have-empty-code (:perspective account)
                 (if-not-zero   account/IS_PRECOMPILE
                                (begin
                                  (vanishes!   account/HAS_CODE     )
                                  (vanishes!   account/HAS_CODE_NEW )
                                  )))

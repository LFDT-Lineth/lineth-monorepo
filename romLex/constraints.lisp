(module romLex)

(defconstraint initialization (:domain {0})
  (vanishes! CODE_FRAGMENT_INDEX))

(defconstraint cfi-evolution ()
  (or! (will-inc! CFI 1) (will-remain-constant! CFI)))

(defconstraint finalisation (:domain {-1})
  (if-not-zero CFI
               (eq! CFI CODE_FRAGMENT_INDEX_INFTY)))

(defconstraint cfi-rules ()
  (if-zero CFI
           (vanishes! CODE_FRAGMENT_INDEX_INFTY)
           (begin (will-inc! CFI 1)
                  (will-remain-constant! CODE_FRAGMENT_INDEX_INFTY)
                  ;;TODO: add lexicographic ordering
                  )))



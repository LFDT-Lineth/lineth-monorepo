(defpurefun (counter-constant col ct) 
    (if-not-zero ct
        (remained-constant! col)))

(defpurefun (increment-by-at-most-one col)
    (or! (will-eq!  col col)
         (will-inc! col 1)))

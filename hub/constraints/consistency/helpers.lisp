(defpurefun ((first-occurrence-of :binary) (BIT :binary) COL) (* BIT
                                                                 (if (force-bool (prev BIT))
                                                                   ;; BIT[i - 1] = 0
                                                                   1
                                                                   ;; BIT[i - 1] = 1
                                                                   (is-not-zero (remained-constant! COL)))))

(defpurefun ((repeat-occurrence-of :binary) (BIT :binary) COL) (* BIT
                                                                  (prev BIT)
                                                                  (is-zero (remained-constant! COL))))

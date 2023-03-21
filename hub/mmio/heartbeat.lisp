(module mmio)


(defunalias if-zero-else if-zero)


;; 1 TODO remove
(defconstraint fast-is-binary ()
  (vanishes (* FAST (- 1 FAST))))

;; 2
(defconstraint micro-stamp-starts-at-zero (:domain {0})
  (vanishes MICRO_STAMP))

;; 3
(defconstraint micro-stamp-non-decreasing ()
  (if-not-zero (remains-constant MICRO_STAMP) (inc MICRO_STAMP 1)))

;; 4
(defconstraint zero-rows ()
  (if-zero MICRO_STAMP
           (begin (vanishes FAST)
            (vanishes COUNTER))))

;; 5
(defconstraint micro-stamp-not-zero (:guard MICRO_STAMP)
               (if-zero-else FAST
                             ;; FAST == 0
                             (begin (if-eq-else CT 15
                                                ;; CT == 15
                                                (begin (will-eq CT 0)
                                                       (inc MICRO_STAMP 1))
                                                ;; CT != 15
                                                (begin (remains-constant FAST)
                                                       (remains-constant MICRO_STAMP)
                                                       (inc COUNTER 1))))
                                        ;FAST == 1
                             (begin (vanishes CT)
                                    (remains-constant CT)
                                    (inc MICRO_STAMP 1))))

;; 6
(defconstraint last-row (:domain {-1} :guard MICRO_STAMP)
  (if-zero FAST
           (eq CT 15)))

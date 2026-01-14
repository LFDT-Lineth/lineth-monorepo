(module mmio)

;;;;;;;;;;;;;;;;;;;;;;;;;
;;                     ;;
;;  Surgical Patterns  ;;
;;                     ;;
;;;;;;;;;;;;;;;;;;;;;;;;;

;; Excision
(defpurefun    (excision    target target_new
                            target_byte
                            accumulator
                            pow
                            target_marker
                            size
                            bit1
                            bit2
                            counter)
               (begin (plateau bit1 target_marker counter)
                      (plateau bit2 (+ target_marker size) counter)
                      (isolate-chunk accumulator target_byte bit1 bit2 counter)
                      (power pow bit2 counter)
                      (if-eq counter LLARGEMO
                             (eq! target_new
                                  (- target (* accumulator pow))))))

;; [1 => 1 Padded]
(defpurefun    (one-to-one-padded    target
                                     source_byte
                                     accumulator
                                     pow
                                     source_marker
                                     target_marker
                                     size
                                     bit1
                                     bit2
                                     bit3
                                     counter)
               (begin (plateau bit1 source_marker counter)
                      (plateau bit2 (+ source_marker size) counter)
                      (plateau bit3 (+ target_marker size) counter)
                      (isolate-chunk accumulator source_byte bit1 bit2 counter)
                      (power pow bit3 counter)
                      (if-eq counter LLARGEMO
                             (eq! target (* accumulator pow)))))

;; [2 => 1 Padded]
(defpurefun    (two-to-one-padded    target
                                     source1_byte
                                     source2_byte
                                     accumulator1
                                     accumulator2
                                     pow1
                                     pow2
                                     source1_marker
                                     target_marker
                                     size
                                     bit1
                                     bit2
                                     bit3
                                     bit4
                                     counter)
               (begin (plateau bit1
                               source1_marker
                               counter)
                      (plateau bit2
                               (+ source1_marker (- size LLARGE))
                               counter)
                      (plateau bit3
                               (- (+ target_marker LLARGE) source1_marker)
                               counter)
               (plateau bit4 (+ target_marker size) counter)
               (isolate-suffix accumulator1 source1_byte bit1 counter)
               (isolate-prefix accumulator2 source2_byte bit2 counter)
               (power pow1 bit3 counter)
               (power pow2 bit4 counter)
               (if-eq counter LLARGEMO
                      (eq! target
                           (+ (* accumulator1 pow1) (* accumulator2 pow2))))))

;; [1 Partial => 1]
(defpurefun (one-partial-to-one target
                                target_new
                                source_byte
                                target_byte
                                accumulator1
                                accumulator2
                                pow
                                source_marker
                                target_marker
                                size
                                bit1
                                bit2
                                bit3
                                bit4
                                counter)
            (begin (plateau bit1 target_marker counter)
                   (plateau bit2 (+ target_marker size) counter)
                   (plateau bit3 source_marker counter)
                   (plateau bit4 (+ source_marker size) counter)
                   (isolate-chunk accumulator1 target_byte bit1 bit2 counter)
                   (isolate-chunk accumulator2 source_byte bit3 bit4 counter)
                   (power pow bit2 counter)
                   (if-eq counter LLARGEMO
                          (eq! target_new
                               (+ target
                                  (* (- accumulator2 accumulator1) pow))))))

;; [1 Partial => 2]
(defpurefun (one-partial-to-two target1
                                target2
                                target1_new
                                target2_new
                                source_byte
                                target1_byte
                                target2_byte
                                accumulator1
                                accumulator2
                                accumulator3
                                accumulator4
                                pow
                                source_marker
                                target1_marker
                                size
                                bit1
                                bit2
                                bit3
                                bit4
                                bit5
                                counter)
            (begin (plateau bit1 target1_marker counter)
                   (plateau bit2
                            (- (+ target1_marker size) LLARGE)
                            counter)
                   (plateau bit3 source_marker counter)
                   (plateau bit4
                            (- (+ source_marker LLARGE) target1_marker)
                            counter)
                   (plateau bit5 (+ source_marker size) counter)
                   (isolate-suffix accumulator1 target1_byte bit1 counter)
                   (isolate-prefix accumulator2 target2_byte bit2 counter)
                   (isolate-chunk accumulator3 source_byte bit3 bit4 counter)
                   (isolate-chunk accumulator4 source_byte bit4 bit5 counter)
                   (power pow bit2 counter)
                   (if-eq counter LLARGEMO
                          (begin (eq! target1_new
                                      (+ target1 (- accumulator3 accumulator1)))
                                 (eq! target2_new
                                      (+ target2
                                         (* (- accumulator4 accumulator2) pow)))))))

;; [2 Partial => 1]

;; This is just a way to cast an intermediate result, as the current constraint were creating a i354 which makes the splitting huge.

(defpurefun (two-partial-to-one target
                                target_new
                                source1_byte
                                source2_byte
                                target_byte
                                accumulator1
                                accumulator2
                                accumulator3
                                pow1
                                pow2
                                source_marker
                                target_marker
                                size
                                bit1
                                bit2
                                bit3
                                bit4
                                counter
                                cast_interemediate_result)
            (begin (plateau bit1 source_marker counter)
                   (plateau bit2
                            (- (+ source_marker size) LLARGE)
                            counter)
                   (plateau bit3 target_marker counter)
                   (plateau bit4 (+ target_marker size) counter)
                   (isolate-suffix accumulator1 source1_byte bit1 counter)
                   (isolate-prefix accumulator2 source2_byte bit2 counter)
                   (isolate-chunk accumulator3 target_byte bit3 bit4 counter)
                   (power pow1 bit4 counter)
                   (antipower pow2 bit2 counter)
                   (if-eq counter LLARGEMO
                            (eq! target_new
                                 (+ target
                                    (* (- (+ cast_interemediate_result accumulator2)
                                          accumulator3)
                                       pow1))))))

;; original constraint:
;; (defpurefun (two-partial-to-one target
;;                                 target_new
;;                                 source1_byte
;;                                 source2_byte
;;                                 target_byte
;;                                 accumulator1
;;                                 accumulator2
;;                                 accumulator3
;;                                 pow1
;;                                 pow2
;;                                 source_marker
;;                                 target_marker
;;                                 size
;;                                 bit1
;;                                 bit2
;;                                 bit3
;;                                 bit4
;;                                 counter)
;;             (begin (plateau bit1 source_marker counter)
;;                    (plateau bit2
;;                             (- (+ source_marker size) LLARGE)
;;                             counter)
;;                    (plateau bit3 target_marker counter)
;;                    (plateau bit4 (+ target_marker size) counter)
;;                    (isolate-suffix accumulator1 source1_byte bit1 counter)
;;                    (isolate-prefix accumulator2 source2_byte bit2 counter)
;;                    (isolate-chunk accumulator3 target_byte bit3 bit4 counter)
;;                    (power pow1 bit4 counter)
;;                    (antipower pow2 bit2 counter)
;;                    (if-eq counter LLARGEMO
;;                             (eq! target_new
;;                                  (+ target
;;                                     (* (- (+ (* accumulator1 pow2) accumulator2)
;;                                           accumulator3)
;;                                        pow1))))))

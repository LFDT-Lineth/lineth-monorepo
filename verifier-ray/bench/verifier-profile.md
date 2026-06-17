# Verifier R5 Profiling

This report is generated from the shared `src/main.zig` R5 verifier path and the shared `arithmetization/src/main/riscv/main.zkc` interpreter. The parser streams `zkc exec` output directly and does not store the full instruction trace.

| Case | Name | Mode | Input | Total cycles | Verifier cycles | Transcript cycles | Vanishing cycles | Poseidon2 compressions | Top instructions |
| ---: | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| 0 | Vanishing/BooleanColumn | profiled | valid | 165888 | 165810 | 159353 | 6351 | 5 | ADDI 23297, ADD 21108, AND 15936, SLLI 12170, SRLI 10789, BEQ 10782, JAL 9460, ADDIW 8715, SRAI 7770, SLTU 7466 |
| 1 | Vanishing/Fibonacci | profiled | valid | 262878 | 262800 | 254144 | 8550 | 8 | ADDI 36628, ADD 33530, AND 25406, SLLI 19423, BEQ 17229, SRLI 17217, JAL 15113, ADDIW 13944, SRAI 12432, SLTU 11854 |
| 2 | Vanishing/GeometricProgression | profiled | valid | 262501 | 262423 | 254168 | 8149 | 8 | ADDI 36528, ADD 33524, AND 25394, SLLI 19417, BEQ 17228, SRLI 17223, JAL 15119, ADDIW 13944, SRAI 12432, SLTU 11842 |
| 3 | Vanishing/ConditionalCounter | profiled | valid | 293663 | 293585 | 285880 | 7599 | 9 | ADDI 40763, ADD 37545, AND 28494, SLLI 21819, SRLI 19390, BEQ 19377, JAL 17019, ADDIW 15687, SRAI 13986, SLTU 13248 |
| 4 | Vanishing/PythagoreanTriplet | profiled | valid | 326824 | 326746 | 317615 | 8995 | 10 | ADDI 45407, ADD 41797, AND 31684, SLLI 24262, SRLI 21564, BEQ 21528, JAL 18921, ADDIW 17430, SRAI 15540, SLTU 14744 |
| 5 | Vanishing/DynamicFibonacci | profiled | valid | 263319 | 263236 | 254144 | 8980 | 8 | ADDI 36546, ADD 33608, AND 25440, SLLI 19466, BEQ 17236, SRLI 17236, JAL 15117, ADDIW 13944, SRAI 12432, SLTU 11888 |
| 6 | Vanishing/ConstantColumn | profiled | valid | 165317 | 165239 | 159326 | 5807 | 5 | ADDI 23239, ADD 21023, AND 15902, SLLI 12146, BEQ 10783, SRLI 10772, JAL 9453, ADDIW 8715, SRAI 7770, SLTU 7432 |
| 7 | Vanishing/ForwardShiftConstant | profiled | valid | 261870 | 261792 | 254221 | 7465 | 8 | ADDI 36421, ADD 33457, AND 25360, SLLI 19413, SRLI 17246, BEQ 17227, JAL 15132, ADDIW 13944, SRAI 12432, SLTU 11808 |
| 8 | Vanishing/BooleanCube | profiled | valid | 264324 | 264244 | 254463 | 9675 | 8 | ADDI 36926, ADD 33708, AND 25468, SLLI 19474, SRLI 17258, BEQ 17235, JAL 15133, ADDIW 13944, SRAI 12432, SLTU 11916 |
| 9 | Vanishing/LinearCombination | profiled | valid | 198457 | 198379 | 191031 | 7242 | 6 | ADDI 27784, ADD 25273, AND 19104, SLLI 14589, BEQ 12937, SRLI 12928, JAL 11344, ADDIW 10458, SRAI 9324, SLTU 8940 |
| 10 | Vanishing/LargeFibonacci | profiled | valid | 485572 | 485494 | 476016 | 9342 | 15 | ADDI 67096, ADD 62234, AND 47336, SLLI 36287, SRLI 32301, BEQ 32267, JAL 28362, ADDIW 26145, SRAI 23310, SLTU 21926 |
| 11 | Vanishing/MultipleVanishingsSameRatio | profiled | valid | 167549 | 167463 | 159157 | 8200 | 5 | ADDI 23516, ADD 21285, AND 16016, SLLI 12217, SRLI 10791, BEQ 10786, JAL 9456, ADDIW 8715, SRAI 7770, LWU 7633 |
| 12 | Vanishing/MixedRatioVanishings | profiled | valid | 169202 | 169116 | 159318 | 9692 | 5 | ADDI 23886, ADD 21442, AND 16090, SLLI 12247, SRLI 10792, BEQ 10787, JAL 9451, ADDIW 8715, LWU 7795, SRAI 7770 |
| 13 | Vanishing/MultiModule | profiled | valid | 458375 | 458296 | 445269 | 12891 | 14 | ADDI 63897, ADD 58541, AND 44384, SLLI 33941, SRLI 30152, BEQ 30145, JAL 26465, ADDIW 24402, SRAI 21756, SLTU 20668 |
| 14 | Vanishing/ManualCancellation | profiled | valid | 166385 | 166307 | 159309 | 6892 | 5 | ADDI 23446, ADD 21111, AND 15948, SLLI 12171, BEQ 10785, SRLI 10773, JAL 9449, ADDIW 8715, SRAI 7770, SLTU 7478 |
| 15 | Vanishing/PrecomputedSelector | profiled | valid | 165975 | 165897 | 159369 | 6422 | 5 | ADDI 23322, ADD 21113, AND 15936, SLLI 12174, SRLI 10797, BEQ 10783, JAL 9464, ADDIW 8715, SRAI 7770, SLTU 7466 |
| 16 | Vanishing/CellLeaf | profiled | valid | 165455 | 165381 | 159443 | 5832 | 5 | ADDI 23269, ADD 21032, AND 15902, SLLI 12161, SRLI 10788, BEQ 10786, JAL 9461, ADDIW 8715, SRAI 7770, SLTU 7432 |
| 17 | Vanishing/CoinScaled | profiled | valid | 198566 | 198488 | 191082 | 7300 | 6 | ADDI 27872, ADD 25274, AND 19104, SLLI 14588, SRLI 12944, BEQ 12935, JAL 11350, ADDIW 10458, SRAI 9324, SLTU 8940 |
| 18 | Vanishing/ThreeStepRecurrence | profiled | valid | 263865 | 263787 | 254180 | 9501 | 8 | ADDI 36818, ADD 33625, AND 25446, SLLI 19455, SRLI 17241, BEQ 17231, JAL 15122, ADDIW 13944, SRAI 12432, SLTU 11894 |
| 19 | Vanishing/Quartic | profiled | valid | 458400 | 458320 | 444570 | 13614 | 14 | ADDI 63647, ADD 58619, AND 44418, SLLI 33977, BEQ 30135, SRLI 30117, JAL 26447, ADDIW 24402, SRAI 21756, SLTU 20702 |
| 20 | Vanishing/LeftPadDynamic | profiled | valid | 262173 | 262090 | 254176 | 7802 | 8 | ADDI 36312, ADD 33524, AND 25394, SLLI 19445, SRLI 17243, BEQ 17233, JAL 15125, ADDIW 13944, SRAI 12432, SLTU 11842 |
| 21 | Vanishing/CubicWithBackShift | profiled | valid | 456045 | 455963 | 444411 | 11416 | 14 | ADDI 63138, ADD 58397, AND 44310, SLLI 33945, SRLI 30163, BEQ 30125, JAL 26474, ADDIW 24402, SRAI 21756, SLTU 20594 |
| 22 | Vanishing/MixedHighRatio | profiled | valid | 1226752 | 1226669 | 1205114 | 21419 | 38 | ADDI 168756, ADD 157567, AND 119860, SLLI 91923, SRLI 81806, BEQ 81709, JAL 71843, ADDIW 66234, SRAI 59052, SLTU 55488 |
| 23 | Vanishing/MultiModuleHighRatio | profiled | valid | 750816 | 750733 | 730522 | 20075 | 23 | ADDI 104198, ADD 96068, AND 72866, SLLI 55769, SRLI 49522, BEQ 49495, JAL 43472, ADDIW 40089, SRAI 35742, SLTU 33904 |
| 24 | Vanishing/SizeThirtyTwoCubic | profiled | valid | 1692376 | 1692296 | 1680139 | 12021 | 53 | ADDI 232038, ADD 217816, AND 166330, SLLI 127705, SRLI 113960, BEQ 113906, JAL 100177, ADDIW 92379, SRAI 82362, SLTU 76548 |
| 25 | Vanishing/LargeForwardShift | profiled | valid | 263776 | 263698 | 254225 | 9367 | 8 | ADDI 36801, ADD 33630, AND 25440, SLLI 19460, SRLI 17260, BEQ 17231, JAL 15133, ADDIW 13944, SRAI 12432, SLTU 11888 |
| 26 | Vanishing/BackAndForwardShift | profiled | valid | 263630 | 263552 | 254213 | 9233 | 8 | ADDI 36736, ADD 33627, AND 25440, SLLI 19457, SRLI 17254, BEQ 17230, JAL 15130, ADDIW 13944, SRAI 12432, SLTU 11888 |
| 27 | Vanishing/DynamicQuadratic | profiled | valid | 261856 | 261774 | 254192 | 7470 | 8 | ADDI 36204, ADD 33521, AND 25388, SLLI 19443, SRLI 17248, BEQ 17231, JAL 15129, ADDIW 13944, SRAI 12432, SLTU 11836 |
| 28 | Vanishing/QuarticWithBackShift | profiled | valid | 840889 | 840807 | 824717 | 15954 | 26 | ADDI 115892, ADD 107918, AND 82062, SLLI 62908, SRLI 55957, BEQ 55915, JAL 49147, ADDIW 45318, SRAI 40404, SLTU 38018 |
| 29 | LocalVanishing/SingleColumnFirstRowZero | profiled | valid | 167748 | 167663 | 159357 | 8200 | 5 | ADDI 23552, ADD 21279, AND 16013, SLLI 12239, SRLI 10859, BEQ 10815, JAL 9462, ADDIW 8715, SRAI 7770, LWU 7595 |
| 30 | LocalVanishing/SingleColumnLastRowZero | profiled | valid | 167778 | 167693 | 159385 | 8202 | 5 | ADDI 23553, ADD 21286, AND 16013, SLLI 12246, SRLI 10873, BEQ 10815, JAL 9469, ADDIW 8715, SRAI 7770, LWU 7595 |
| 31 | LocalVanishing/ShiftedColumnFirstRowZero | profiled | valid | 167710 | 167625 | 159317 | 8202 | 5 | ADDI 23553, ADD 21269, AND 16013, SLLI 12229, SRLI 10839, BEQ 10815, JAL 9452, ADDIW 8715, SRAI 7770, LWU 7595 |
| 32 | LocalVanishing/TwoColumnsEqualAtFirstRow | profiled | valid | 167670 | 167585 | 159145 | 8334 | 5 | ADDI 23449, ADD 21279, AND 16019, SLLI 12242, SRLI 10844, BEQ 10815, JAL 9454, ADDIW 8715, SRAI 7770, LWU 7621 |
| 33 | LocalVanishing/MultipleConstraintsSameModule | profiled | valid | 172814 | 172733 | 159317 | 13310 | 5 | ADDI 24372, ADD 21777, AND 16238, SLLI 12383, SRLI 10925, BEQ 10853, JAL 9452, ADDIW 8715, LWU 8117, SRAI 7770 |
| 34 | LocalVanishing/SecondRowConstraint | profiled | valid | 167936 | 167851 | 159337 | 8408 | 5 | ADDI 23597, ADD 21281, AND 16019, SLLI 12240, SRLI 10852, BEQ 10816, JAL 9457, ADDIW 8715, SRAI 7770, LWU 7613 |
| 35 | LocalVanishing/CellEquality | profiled | valid | 168049 | 167964 | 159430 | 8428 | 5 | ADDI 23625, ADD 21284, AND 16019, SLLI 12249, SRLI 10856, BEQ 10819, JAL 9459, ADDIW 8715, SRAI 7770, LWU 7625 |
| 36 | LocalVanishing/CoinScaled | profiled | valid | 200212 | 200127 | 191001 | 9020 | 6 | ADDI 28064, ADD 25424, AND 19181, SLLI 14636, SRLI 12972, BEQ 12967, JAL 11331, ADDIW 10458, SRAI 9324, LWU 9032 |
| 37 | LocalVanishing/MultipleAnchorsSharedColumn | profiled | valid | 178465 | 178380 | 159297 | 18977 | 5 | ADDI 25320, ADD 22299, AND 16481, SLLI 12547, SRLI 11010, BEQ 10893, JAL 9447, ADDIW 8715, LWU 8687, SLTU 8005 |
| 38 | LocalVanishing/ConstantSubtraction | profiled | valid | 167978 | 167893 | 159381 | 8406 | 5 | ADDI 23596, ADD 21292, AND 16019, SLLI 12251, SRLI 10874, BEQ 10816, JAL 9468, ADDIW 8715, SRAI 7770, LWU 7613 |
| 39 | LocalVanishing/WrapAroundShift | profiled | valid | 167738 | 167653 | 159345 | 8202 | 5 | ADDI 23553, ADD 21276, AND 16013, SLLI 12236, SRLI 10853, BEQ 10815, JAL 9459, ADDIW 8715, SRAI 7770, LWU 7595 |
| 40 | LocalVanishing/ProductIsZero | profiled | valid | 265949 | 265864 | 254238 | 11520 | 8 | ADDI 37060, ADD 33869, AND 25545, SLLI 19536, SRLI 17302, BEQ 17268, JAL 15123, ADDIW 13944, SRAI 12432, LWU 12028 |
| 41 | LocalVanishing/CellAndCoin | profiled | valid | 232050 | 231965 | 222757 | 9102 | 7 | ADDI 32402, ADD 29536, AND 22309, SLLI 17076, SRLI 15179, BEQ 15119, JAL 13252, ADDIW 12201, SRAI 10878, SLTU 10449 |
| 42 | LocalVanishing/ThreeColumnLinear | profiled | valid | 199742 | 199657 | 191078 | 8473 | 6 | ADDI 27924, ADD 25386, AND 19153, SLLI 14658, SRLI 13020, BEQ 12968, JAL 11357, ADDIW 10458, SRAI 9324, LWU 8988 |
| 43 | LocalVanishing/MultiAnchorMultiColumn | profiled | valid | 172812 | 172727 | 159176 | 13445 | 5 | ADDI 24270, ADD 21795, AND 16244, SLLI 12404, SRLI 10946, BEQ 10853, JAL 9462, ADDIW 8715, LWU 8143, SLTU 7770 |
| 44 | LocalVanishing/CubeAtFirstRow | profiled | valid | 460442 | 460357 | 444626 | 15595 | 14 | ADDI 63921, ADD 58809, AND 44501, SLLI 34065, SRLI 30216, BEQ 30168, JAL 26462, ADDIW 24402, SRAI 21756, SLTU 20783 |
| 45 | LocalVanishing/MultiModule | profiled | valid | 335252 | 335175 | 318383 | 16656 | 10 | ADDI 46964, ADD 42550, AND 32026, SLLI 24486, SRLI 21710, BEQ 21614, JAL 18915, ADDIW 17430, SRAI 15540, LWU 15218 |
| 46 | LocalVanishing/DynamicFirstRowZero | profiled | valid | 263648 | 263563 | 254136 | 9315 | 8 | ADDI 36458, ADD 33677, AND 25465, SLLI 19497, SRLI 17288, BEQ 17264, JAL 15116, ADDIW 13944, SRAI 12432, SLTU 11911 |
| 47 | LocalVanishing/DynamicShifted | profiled | valid | 263759 | 263672 | 254152 | 9408 | 8 | ADDI 36484, ADD 33681, AND 25465, SLLI 19501, SRLI 17296, BEQ 17265, JAL 15120, ADDIW 13944, SRAI 12432, SLTU 11911 |
| 48 | LocalVanishing/DynamicProductIsZero | profiled | valid | 488988 | 488903 | 476110 | 12651 | 15 | ADDI 67419, ADD 62651, AND 47509, SLLI 36443, SRLI 32405, BEQ 32312, JAL 28376, ADDIW 26145, SRAI 23310, SLTU 22097 |
| 49 | LogDerivativeSumCompiler/SingleFractionAllOnes | profiled | valid | 336561 | 336472 | 318211 | 18125 | 10 | ADDI 47060, ADD 42671, AND 32084, SLLI 24568, SRLI 21739, BEQ 21616, JAL 18926, ADDIW 17430, SRAI 15540, LWU 15414 |
| 50 | LogDerivativeSumCompiler/PartialFilter | profiled | valid | 368566 | 368477 | 349603 | 18738 | 11 | ADDI 51379, ADD 46815, AND 35246, SLLI 26968, SRLI 23857, BEQ 23766, JAL 20800, ADDIW 19173, SRAI 17094, LWU 16841 |
| 51 | LogDerivativeSumCompiler/AllZeroFilter | profiled | valid | 368541 | 368452 | 349580 | 18736 | 11 | ADDI 51378, ADD 46809, AND 35246, SLLI 26962, SRLI 23845, BEQ 23766, JAL 20794, ADDIW 19173, SRAI 17094, LWU 16841 |
| 52 | LogDerivativeSumCompiler/FilterMasksZeroDenominator | profiled | valid | 368790 | 368699 | 349896 | 18667 | 11 | ADDI 51452, ADD 46829, AND 35246, SLLI 26983, SRLI 23879, BEQ 23768, JAL 20812, ADDIW 19173, SRAI 17094, LWU 16841 |
| 53 | LogDerivativeSumCompiler/Packing4Fractions | profiled | valid | 936304 | 936219 | 889578 | 46505 | 28 | ADDI 130525, ADD 119038, AND 89630, SLLI 68624, SRLI 60713, BEQ 60420, JAL 52966, ADDIW 48804, SRAI 43512, LWU 42718 |
| 54 | LogDerivativeSumCompiler/MultiModuleBucketing | profiled | valid | 705214 | 705127 | 667405 | 37586 | 21 | ADDI 98582, ADD 89453, AND 67330, SLLI 51514, SRLI 45568, BEQ 45366, JAL 39705, ADDIW 36603, SRAI 32634, LWU 32251 |
| 55 | LogDerivativeSumCompiler/SizeOneModule | profiled | valid | 172295 | 172216 | 159774 | 12335 | 5 | ADDI 24345, ADD 21639, AND 16182, SLLI 12387, SRLI 10935, BEQ 10867, JAL 9463, ADDIW 8715, LWU 8037, SRAI 7770 |
| 56 | LogDerivativeSumCompiler/ConditionalLookupShape | profiled | valid | 577921 | 577830 | 541281 | 36413 | 17 | ADDI 81018, ADD 73093, AND 54805, SLLI 41932, SRLI 37041, BEQ 36773, JAL 32177, ADDIW 29634, LWU 26811, SRAI 26418 |
| 57 | LogDerivativeSumCompiler/ManyFractions | profiled | valid | 1147826 | 1147734 | 1079810 | 67788 | 34 | ADDI 160300, ADD 145646, AND 109338, SLLI 83612, SRLI 73792, BEQ 73419, JAL 64289, ADDIW 59262, LWU 53042, SRAI 52836 |
| 58 | LogDerivativeSumCompiler/SizeTwoModule | profiled | valid | 208674 | 208585 | 191310 | 17169 | 6 | ADDI 29408, ADD 26231, AND 19538, SLLI 14925, SRLI 13104, BEQ 13019, JAL 11348, ADDIW 10458, LWU 9942, SLTU 9370 |
| 59 | LogDerivativeSumCompiler/MultipleQueries | profiled | valid | 542486 | 542399 | 508510 | 33753 | 16 | ADDI 75944, ADD 68647, AND 51508, SLLI 39447, SRLI 34858, BEQ 34605, JAL 30282, ADDIW 27888, LWU 25094, SRAI 24864 |
| 60 | LogDerivativeSumCompiler/VectorDenominator | profiled | valid | 367871 | 367782 | 349591 | 18055 | 11 | ADDI 51273, ADD 46732, AND 35212, SLLI 26948, SRLI 23848, BEQ 23765, JAL 20797, ADDIW 19173, SRAI 17094, LWU 16765 |
| 61 | LogDerivativeSumCompiler/AllFiltersOnesPacked | profiled | valid | 794733 | 794647 | 761728 | 32783 | 24 | ADDI 110167, ADD 101450, AND 76564, SLLI 58590, SRLI 51873, BEQ 51730, JAL 45378, ADDIW 41832, SRAI 37296, LWU 35980 |
| 62 | Lookup/SingleColumnNoFilters | profiled | valid | 705465 | 705378 | 667962 | 37280 | 21 | ADDI 98756, ADD 89409, AND 67308, SLLI 51518, SRLI 45591, BEQ 45369, JAL 39715, ADDIW 36603, SRAI 32634, LWU 32215 |
| 63 | Lookup/FilterOnIncluded | profiled | valid | 609388 | 609297 | 572838 | 36323 | 18 | ADDI 85339, ADD 77128, AND 57924, SLLI 44282, SRLI 39098, BEQ 38922, JAL 34024, ADDIW 31374, LWU 28160, SRAI 27972 |
| 64 | Lookup/FilterOnIncluding | profiled | valid | 738963 | 738874 | 699874 | 38864 | 22 | ADDI 103404, ADD 93687, AND 70516, SLLI 53977, SRLI 47776, BEQ 47524, JAL 41618, ADDIW 38346, SRAI 34188, LWU 33766 |
| 65 | Lookup/DoubleConditional | profiled | valid | 771424 | 771335 | 731712 | 39487 | 23 | ADDI 107944, ADD 97837, AND 73678, SLLI 56380, SRLI 49904, BEQ 49677, JAL 43497, ADDIW 40089, SRAI 35742, LWU 35185 |
| 66 | Lookup/MultiColumn | profiled | valid | 770704 | 770615 | 731676 | 38803 | 23 | ADDI 107837, ADD 97748, AND 73644, SLLI 56354, SRLI 49883, BEQ 49676, JAL 43488, ADDIW 40089, SRAI 35742, LWU 35109 |
| 67 | Lookup/SharedTable | profiled | valid | 915893 | 915806 | 860941 | 54729 | 27 | ADDI 129136, ADD 115604, AND 86852, SLLI 66401, SRLI 58630, BEQ 58371, JAL 51024, ADDIW 47061, LWU 42161, SRAI 41958 |
| 68 | Lookup/DistinctTables | profiled | valid | 1060350 | 1060258 | 988443 | 71679 | 31 | ADDI 149756, ADD 133595, AND 100124, SLLI 76522, SRLI 67494, BEQ 67074, JAL 58610, ADDIW 54039, LWU 49353, SRAI 48174 |
| 69 | Lookup/MultiColumnFilterOnIncluding | profiled | valid | 772102 | 772013 | 731491 | 40386 | 23 | ADDI 107941, ADD 97932, AND 73724, SLLI 56404, SRLI 49891, BEQ 49677, JAL 43487, ADDIW 40089, SRAI 35742, LWU 35317 |
| 70 | Lookup/RepeatedValueInTable | profiled | valid | 705347 | 705260 | 667846 | 37278 | 21 | ADDI 98755, ADD 89380, AND 67308, SLLI 51489, SRLI 45533, BEQ 45369, JAL 39686, ADDIW 36603, SRAI 32634, LWU 32215 |
| 71 | Lookup/ShiftedAColumn | profiled | valid | 901780 | 901690 | 859680 | 41874 | 27 | ADDI 126392, ADD 114379, AND 86270, SLLI 66075, SRLI 58540, BEQ 58276, JAL 51072, ADDIW 47061, SRAI 41958, LWU 40811 |
| 72 | Lookup/ShiftedBColumn | profiled | valid | 705520 | 705433 | 668022 | 37275 | 21 | ADDI 98756, ADD 89424, AND 67308, SLLI 51533, SRLI 45621, BEQ 45369, JAL 39730, ADDIW 36603, SRAI 32634, LWU 32215 |
| 73 | Lookup/MultipleAFragments | profiled | valid | 915941 | 915854 | 860989 | 54729 | 27 | ADDI 129136, ADD 115616, AND 86852, SLLI 66413, SRLI 58654, BEQ 58371, JAL 51036, ADDIW 47061, LWU 42161, SRAI 41958 |
| 74 | Lookup/WidthThree | profiled | valid | 675715 | 675624 | 636735 | 38753 | 20 | ADDI 94595, ADD 85583, AND 64306, SLLI 49160, SRLI 43421, BEQ 43232, JAL 37809, ADDIW 34860, LWU 31178, SRAI 31080 |
| 75 | Lookup/SizeOne | profiled | valid | 375931 | 375848 | 351023 | 24689 | 11 | ADDI 52945, ADD 47343, AND 35492, SLLI 27160, SRLI 24009, BEQ 23868, JAL 20806, ADDIW 19173, LWU 17419, SRAI 17094 |
| 76 | Lookup/PrecomputedTable | profiled | valid | 673530 | 673443 | 636032 | 37275 | 20 | ADDI 94299, ADD 85315, AND 64180, SLLI 49109, SRLI 43420, BEQ 43216, JAL 37813, ADDIW 34860, SRAI 31080, LWU 30872 |
| 77 | Lookup/RepeatedSValues | profiled | valid | 705452 | 705365 | 667962 | 37267 | 21 | ADDI 98756, ADD 89409, AND 67308, SLLI 51518, SRLI 45591, BEQ 45369, JAL 39715, ADDIW 36603, SRAI 32634, LWU 32215 |
| 78 | Lookup/EmptySelected | profiled | valid | 705839 | 705756 | 667786 | 37834 | 21 | ADDI 98713, ADD 89491, AND 67342, SLLI 51541, SRLI 45594, BEQ 45369, JAL 39716, ADDIW 36603, SRAI 32634, LWU 32299 |
| 79 | RangeCheckCompiler/Basic | profiled | valid | 1087038 | 1086949 | 1048023 | 38790 | 33 | ADDI 150890, ADD 138609, AND 104921, SLLI 80418, SRLI 71405, BEQ 71145, JAL 62401, ADDIW 57522, SRAI 51282, SLTU 49002 |
| 80 | RangeCheckCompiler/SharedBound | profiled | valid | 903322 | 903236 | 859706 | 43394 | 27 | ADDI 126574, ADD 114586, AND 86365, SLLI 66139, SRLI 58575, BEQ 58274, JAL 51079, ADDIW 47064, SRAI 41958, LWU 41041 |
| 81 | RangeCheckCompiler/DistinctBounds | profiled | valid | 1461635 | 1461544 | 1399050 | 62358 | 44 | ADDI 203907, ADD 185841, AND 140394, SLLI 107462, SRLI 95199, BEQ 94903, JAL 83144, ADDIW 76698, SRAI 68376, LWU 66012 |
| 82 | RangeCheckCompiler/BoundIsPowerOfTwo | profiled | valid | 1308063 | 1307972 | 1269838 | 37998 | 40 | ADDI 180951, ADD 167163, AND 126817, SLLI 97194, SRLI 86358, BEQ 86181, JAL 75587, ADDIW 69723, SRAI 62160, SLTU 59040 |
| 83 | RangeCheckCompiler/BoundIsOne | profiled | valid | 509258 | 509167 | 478127 | 30904 | 15 | ADDI 71644, ADD 64299, AND 48272, SLLI 36947, SRLI 32657, BEQ 32469, JAL 28373, ADDIW 26145, LWU 23473, SRAI 23310 |
| 84 | RangeCheckCompiler/MultiModule | profiled | valid | 1043063 | 1042975 | 987020 | 55819 | 31 | ADDI 146562, ADD 132002, AND 99407, SLLI 76016, SRLI 67205, BEQ 66962, JAL 58568, ADDIW 54036, SRAI 48174, LWU 47645 |
| 85 | RangeCheckCompiler/LargeBound | profiled | valid | 7267797 | 7267706 | 7226419 | 41121 | 228 | ADDI 995031, ADD 935597, AND 715017, SLLI 549163, SRLI 490268, BEQ 490050, JAL 430850, ADDIW 397407, SRAI 354312, SLTU 328768 |
| 86 | RangeCheckCompiler/NonPowerOfTwoBound | profiled | valid | 895445 | 895354 | 857999 | 37219 | 27 | ADDI 124532, ADD 113986, AND 86119, SLLI 65945, SRLI 58458, BEQ 58253, JAL 51037, ADDIW 47064, SRAI 41958, LWU 40395 |
| 87 | RangeCheckCompiler/AllZeros | profiled | valid | 895477 | 895386 | 858031 | 37219 | 27 | ADDI 124532, ADD 113994, AND 86119, SLLI 65953, SRLI 58474, BEQ 58253, JAL 51045, ADDIW 47064, SRAI 41958, LWU 40395 |
| 88 | Vanishing/LagrangeSelectorBoundary | profiled | valid | 167932 | 167847 | 159333 | 8408 | 5 | ADDI 23596, ADD 21280, AND 16019, SLLI 12239, SRLI 10850, BEQ 10816, JAL 9456, ADDIW 8715, SRAI 7770, LWU 7613 |
| 89 | Vanishing/DynamicLagrangeSelectorBoundary | profiled | valid | 168504 | 168417 | 159333 | 8972 | 5 | ADDI 23592, ADD 21358, AND 16053, SLLI 12276, SRLI 10865, BEQ 10822, JAL 9459, ADDIW 8715, SRAI 7770, LWU 7711 |

## Fixture Metadata

| Case | Modules | Dynamic modules | Rounds | Expressions | Buckets | Vanishings | Witness claims | Quotient claims |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 0 | 1 | 0 | 3 | 5 | 1 | 1 | 1 | 1 |
| 1 | 1 | 0 | 3 | 5 | 1 | 1 | 3 | 1 |
| 2 | 1 | 0 | 3 | 5 | 1 | 1 | 2 | 1 |
| 3 | 1 | 0 | 3 | 5 | 1 | 1 | 3 | 1 |
| 4 | 1 | 0 | 3 | 8 | 1 | 1 | 3 | 1 |
| 5 | 1 | 1 | 3 | 5 | 1 | 1 | 3 | 1 |
| 6 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 7 | 1 | 0 | 3 | 3 | 1 | 1 | 2 | 1 |
| 8 | 1 | 0 | 3 | 7 | 1 | 1 | 1 | 2 |
| 9 | 1 | 0 | 3 | 9 | 1 | 1 | 3 | 1 |
| 10 | 1 | 0 | 3 | 5 | 1 | 1 | 3 | 1 |
| 11 | 1 | 0 | 3 | 6 | 1 | 2 | 2 | 1 |
| 12 | 1 | 0 | 3 | 8 | 1 | 2 | 2 | 1 |
| 13 | 2 | 0 | 3 | 8 | 2 | 2 | 2 | 2 |
| 14 | 1 | 0 | 3 | 5 | 1 | 1 | 2 | 1 |
| 15 | 1 | 0 | 3 | 5 | 1 | 1 | 2 | 1 |
| 16 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 17 | 1 | 0 | 4 | 5 | 1 | 1 | 2 | 1 |
| 18 | 1 | 0 | 3 | 5 | 1 | 1 | 3 | 1 |
| 19 | 1 | 0 | 3 | 9 | 1 | 1 | 1 | 4 |
| 20 | 1 | 1 | 3 | 3 | 1 | 1 | 2 | 1 |
| 21 | 1 | 0 | 3 | 7 | 1 | 1 | 2 | 2 |
| 22 | 1 | 0 | 3 | 16 | 2 | 2 | 1 | 6 |
| 23 | 2 | 0 | 3 | 14 | 2 | 2 | 2 | 4 |
| 24 | 1 | 0 | 3 | 7 | 1 | 1 | 1 | 2 |
| 25 | 1 | 0 | 3 | 3 | 1 | 1 | 2 | 1 |
| 26 | 1 | 0 | 3 | 7 | 1 | 1 | 3 | 1 |
| 27 | 1 | 1 | 3 | 5 | 1 | 1 | 1 | 1 |
| 28 | 1 | 0 | 3 | 11 | 1 | 1 | 2 | 4 |
| 29 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 30 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 31 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 32 | 1 | 0 | 3 | 5 | 1 | 1 | 2 | 1 |
| 33 | 1 | 0 | 3 | 6 | 1 | 2 | 1 | 1 |
| 34 | 1 | 0 | 3 | 5 | 1 | 1 | 1 | 1 |
| 35 | 1 | 0 | 3 | 5 | 1 | 1 | 1 | 1 |
| 36 | 1 | 0 | 4 | 7 | 1 | 1 | 1 | 1 |
| 37 | 1 | 0 | 3 | 15 | 1 | 3 | 1 | 1 |
| 38 | 1 | 0 | 3 | 5 | 1 | 1 | 1 | 1 |
| 39 | 1 | 0 | 3 | 3 | 1 | 1 | 1 | 1 |
| 40 | 1 | 0 | 3 | 5 | 1 | 1 | 2 | 2 |
| 41 | 1 | 0 | 4 | 7 | 1 | 1 | 1 | 1 |
| 42 | 1 | 0 | 3 | 7 | 1 | 1 | 3 | 1 |
| 43 | 1 | 0 | 3 | 8 | 1 | 2 | 2 | 1 |
| 44 | 1 | 0 | 3 | 9 | 1 | 1 | 1 | 4 |
| 45 | 2 | 0 | 3 | 6 | 2 | 2 | 2 | 2 |
| 46 | 1 | 1 | 3 | 3 | 1 | 1 | 1 | 1 |
| 47 | 1 | 1 | 3 | 3 | 1 | 1 | 1 | 1 |
| 48 | 1 | 1 | 3 | 5 | 1 | 1 | 2 | 2 |
| 49 | 1 | 0 | 4 | 17 | 1 | 3 | 3 | 1 |
| 50 | 1 | 0 | 4 | 19 | 1 | 3 | 4 | 1 |
| 51 | 1 | 0 | 4 | 19 | 1 | 3 | 4 | 1 |
| 52 | 1 | 0 | 4 | 19 | 1 | 3 | 5 | 1 |
| 53 | 1 | 0 | 4 | 54 | 2 | 6 | 8 | 5 |
| 54 | 2 | 0 | 4 | 36 | 2 | 6 | 7 | 2 |
| 55 | 1 | 0 | 4 | 10 | 1 | 2 | 1 | 1 |
| 56 | 2 | 0 | 4 | 41 | 2 | 6 | 8 | 2 |
| 57 | 1 | 0 | 4 | 91 | 2 | 9 | 13 | 5 |
| 58 | 1 | 0 | 4 | 17 | 1 | 3 | 3 | 1 |
| 59 | 1 | 0 | 4 | 34 | 1 | 6 | 6 | 1 |
| 60 | 1 | 0 | 4 | 17 | 1 | 3 | 4 | 1 |
| 61 | 1 | 0 | 4 | 43 | 2 | 3 | 6 | 5 |
| 62 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 63 | 2 | 0 | 5 | 41 | 2 | 6 | 8 | 2 |
| 64 | 2 | 0 | 5 | 47 | 2 | 6 | 8 | 2 |
| 65 | 2 | 0 | 5 | 49 | 2 | 6 | 9 | 2 |
| 66 | 2 | 0 | 5 | 47 | 2 | 6 | 9 | 2 |
| 67 | 3 | 0 | 5 | 58 | 3 | 9 | 10 | 3 |
| 68 | 4 | 0 | 5 | 78 | 4 | 12 | 14 | 4 |
| 69 | 2 | 0 | 5 | 55 | 2 | 6 | 10 | 2 |
| 70 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 71 | 2 | 0 | 5 | 39 | 3 | 6 | 7 | 4 |
| 72 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 73 | 3 | 0 | 5 | 58 | 3 | 9 | 10 | 3 |
| 74 | 2 | 0 | 5 | 55 | 2 | 6 | 11 | 2 |
| 75 | 2 | 0 | 5 | 20 | 2 | 4 | 2 | 2 |
| 76 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 77 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 78 | 2 | 0 | 5 | 41 | 2 | 6 | 8 | 2 |
| 79 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 80 | 2 | 0 | 5 | 53 | 3 | 6 | 8 | 4 |
| 81 | 3 | 0 | 5 | 73 | 4 | 9 | 12 | 5 |
| 82 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 83 | 2 | 0 | 5 | 29 | 2 | 5 | 4 | 2 |
| 84 | 3 | 0 | 5 | 58 | 3 | 9 | 10 | 3 |
| 85 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 86 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 87 | 2 | 0 | 5 | 39 | 2 | 6 | 7 | 2 |
| 88 | 1 | 0 | 3 | 5 | 1 | 1 | 1 | 1 |
| 89 | 1 | 1 | 3 | 5 | 1 | 1 | 1 | 1 |

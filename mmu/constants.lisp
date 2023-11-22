(module mmu)

(defconst

	;transplants
	RamToRam									601
	ExoToRam 									602
	RamIsExo 									603
	KillingOne 									604
	PushTwoRamToStack							605
	PushOneRamToStack							606
	ExceptionalRamToStack3To2FullFast			607
	PushTwoStackToRam							608									
	StoreXInAThreeRequired 						609
	StoreXInB 									610
	StoreXInC 									611

	;surgeries
	RamLimbExcision								613
	RamToRamSlideChunk							614
	RamToRamSlideOverlappingChunk 				615
	ExoToRamSlideChunk  						616
	ExoToRamSlideOverlappingChunk				618
	PaddedExoFromOne 							619
	PaddedExoFromTwo 							620
	FullExoFromTwo 								621
	FullStackToRam								623
	LsbFromStackToRAM 							624
	FirstFastSecondPadded						625
	FirstPaddedSecondZero						626
	Exceptional_RamToStack_3To2Full 			627
	NA_RamToStack_3To2Full						628
	NA_RamToStack_3To2Padded 					629
	NA_RamToStack_2To2Padded 					630
	NA_RamToStack_2To1FullAndZero 				631
	NA_RamToStack_2To1PaddedAndZero 			632
	NA_RamToStack_1To1PaddedAndZero 			633
	
	; precomputation types
	type1										100	
	type2										200
	type3										300
	type4CC										401  
	type4CD										402
	type4RD										403
	type5										500

	;admissible values of TERNARY
	tern0										0
	tern1										1
	tern2										2

	CALLDATALOAD								53
	CALLDATACOPY								55
	CODECOPY									57
	EXTCODECOPY									60
	RETURNDATACOPY								62

	LIMB_SIZE									16
	SMALL_LIMB_SIZE								4
	LIMB_SIZE_MINUS_ONE							15
	SMALL_LIMB_SIZE_MINUS_ONE 					3)

(defalias
	LLARGE		LIMB_SIZE
	SSMALL		SMALL_LIMB_SIZE
	LLARGEMO	LIMB_SIZE_MINUS_ONE
	SSMALLMO	SMALL_LIMB_SIZE_MINUS_ONE)

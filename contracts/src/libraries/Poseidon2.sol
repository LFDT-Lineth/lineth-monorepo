// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.30;

/**
 * @title Library to perform Poseidon2 hashing with lazy reductions.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 * @notice Optimized version using lazy modular reduction to minimize addmod operations.
 *
 * Key optimizations:
 * - computeSum: Use regular add for 15 additions, single mod at end (saves ~78 gas × 21 rounds)
 * - matMulM4: Use regular add internally, mod only on outputs (saves ~47 gas × 28 calls)
 * - addRoundKey: Skip reduction, let sbox handle it via mulmod (saves ~40 gas × 12 calls)
 * - sumColumns: Accumulate with add, single mod per column
 */
library Poseidon2 {
  /// Thrown when the data is not purely in 32 byte chunks.
  error DataIsNotMod32();

  /// Thrown when the data is empty.
  error DataIsEmpty();

  uint32 private constant R_MOD = 2130706433;
  uint256 private constant DATA_IS_NOT_MOD32_SELECTOR =
    0xc2cab26c00000000000000000000000000000000000000000000000000000000;
  uint256 private constant DATA_IS_EMPTY_SELECTOR = 0x91ea306f00000000000000000000000000000000000000000000000000000000;

  uint256 private constant RK_0_0 = 52691802021506155758914962750280372212207119203515444126415105344946620971042;
  uint256 private constant RK_0_1 = 32207471970256316655474490955553459742787419335289228299095903266455798739660;
  uint256 private constant RK_1_0 = 22163791677048831312463448776400028385347383911100916908889018061663075177430;
  uint256 private constant RK_1_1 = 52176870515245694198691020358647906763460864198395806999480367588806463375956;
  uint256 private constant RK_2_0 = 20057732892593326318373784289292844513497509146673516636435948826170767804894;
  uint256 private constant RK_2_1 = 18178750968836063378915279115560606579387492034215281114134869585318875352069;

  uint256 private constant RK_3 = 271263440;
  uint256 private constant RK_4 = 648183298;
  uint256 private constant RK_5 = 1662653166;
  uint256 private constant RK_6 = 1984135584;
  uint256 private constant RK_7 = 594964655;
  uint256 private constant RK_8 = 108023522;
  uint256 private constant RK_9 = 849546096;
  uint256 private constant RK_10 = 1961993938;
  uint256 private constant RK_11 = 1546336947;
  uint256 private constant RK_12 = 1036613726;
  uint256 private constant RK_13 = 452088758;
  uint256 private constant RK_14 = 275827416;
  uint256 private constant RK_15 = 763236035;
  uint256 private constant RK_16 = 1068717067;
  uint256 private constant RK_17 = 1580958419;
  uint256 private constant RK_18 = 1376393748;
  uint256 private constant RK_19 = 892777736;
  uint256 private constant RK_20 = 1345121022;
  uint256 private constant RK_21 = 908739241;
  uint256 private constant RK_22 = 908871000;
  uint256 private constant RK_23 = 1053550888;

  uint256 private constant RK_24_0 = 7748602703960850726417234176553190502144764796409545305676706613632081374108;
  uint256 private constant RK_24_1 = 14236178139181542176197168604443439317755831764127965411495610808590768539130;
  uint256 private constant RK_25_0 = 4390545490380878999875851257118004037103299792992754784803412502652715488373;
  uint256 private constant RK_25_1 = 15895992674755709583772916119416271803214403026810424648839392700430632374893;
  uint256 private constant RK_26_0 = 37373517675827041221658956101645979913006475784844873469590649853964048342988;
  uint256 private constant RK_26_1 = 46010512812451809471058691124553676654818408969360806522307687423952321374687;

  /**
   * @notice Computes Poseidon2 hash over calldata message (must be 32-byte aligned).
   */
  function hash(bytes calldata _msg) external pure returns (bytes32 poseidon2Hash) {
    assembly {
      let len := _msg.length
      if iszero(len) {
        revert_with_error(DATA_IS_EMPTY_SELECTOR)
      }

      if and(len, 31) {
        revert_with_error(DATA_IS_NOT_MOD32_SELECTOR)
      }

      let q := shr(5, len)
      let ptrMsg := _msg.offset
      for {
        let i := 0
      } lt(i, q) {
        i := add(i, 1)
      } {
        let tmp := calldataload(ptrMsg)
        let a := poseidon2Hash
        let b := tmp
        a, b := permutation(a, b)
        poseidon2Hash := addRoundKeyUint256Reduced(tmp, b)
        ptrMsg := add(ptrMsg, 0x20)
      }

      /**
       * @dev Poseidon2 permutation over a 2-element state (ra, rb).
       *
       * The permutation consists of:
       *  1. Initial external MDS mixing
       *  2. 3 full rounds
       *  3. 21 partial rounds
       *  4. 3 final full rounds
       *
       * Each round follows the Poseidon2 specification for t = 2.
       *
       * @param a First state element (capacity/output lane)
       * @param b Second state element (rate/input lane)
       * @return ra Updated first state element
       * @return rb Updated second state element
       */
      function permutation(a, b) -> ra, rb {
        /*-------------------------------------------------------------*
        | Initial external MDS                                         |
        |                                                              |
        | Ensures early diffusion between a and b before non-linearity |
        *-------------------------------------------------------------*/
        ra, rb := matMulExternalInPlace(a, b)

        /*--------------------------------------------------------------*
        | FULL ROUNDS (3)                                               |
        |                                                               |
        | Each full round:                                              |
        |   1. Add round key to ALL limbs                               |
        |   2. Apply S-box (x ↦ x³) to ALL limbs                        |
        |   3. Apply external MDS                                       |
        *--------------------------------------------------------------*/
        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_0_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_0_1)
        ra, rb := matMulExternalInPlace(ra, rb)

        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_1_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_1_1)
        ra, rb := matMulExternalInPlace(ra, rb)

        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_2_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_2_1)
        ra, rb := matMulExternalInPlace(ra, rb)

        /*--------------------------------------------------------------*
        | PARTIAL ROUNDS (21)                                           |
        |                                                               |
        | Key Poseidon2 optimization:                                   |
        |   - Only FIRST limb is non-linear                             |
        |   - Only FIRST limb gets round key                            |
        |   - Internal MDS provides diffusion                           |
        |                                                               |
        | Each partial round:                                           |
        |   1. Add round key to first limb only                         |
        |   2. Apply S-box to first limb only                           |
        |   3. Compute sum of all limbs (both state elements)           |
        |   4. Apply internal MDS using that sum                        |
        |                                                               | 
        *--------------------------------------------------------------*/
        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_3)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_4)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_5)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_6)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_7)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_8)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_9)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_10)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_11)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_12)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_13)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_14)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_15)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_16)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_17)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_18)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_19)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_20)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_21)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_22)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        ra := addRoundKeyFirstEntryUnreducedAndSbox(ra, RK_23)
        {
          let s := computeSumLazy(ra, rb)
          ra := matMulInternalInPlaceFirstHalf(ra, s)
          rb := matMulInternalInPlaceSecondHalf(rb, s)
        }

        // Final full rounds (3)
        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_24_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_24_1)
        ra, rb := matMulExternalInPlace(ra, rb)

        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_25_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_25_1)
        ra, rb := matMulExternalInPlace(ra, rb)

        ra := addRoundKeyUint256UnreducedAndSbox(ra, RK_26_0)
        rb := addRoundKeyUint256UnreducedAndSbox(rb, RK_26_1)
        ra, rb := matMulExternalInPlace(ra, rb)
      }

      /**
       * @dev Computes sum of all 16 limbs using lazy reduction.
       * Uses regular add for accumulation, single mod at end.
       * Each limb < P < 2^31, sum of 16 limbs < 16P < 2^35.
       * Saves: 15 addmod (120 gas) -> 15 add + 1 mod (50 gas) = 70 gas per call
       */
      function computeSumLazy(a, b) -> s {
        let M := 0xFFFFFFFF
        // Accumulate with regular add - no overflow risk in 256-bit arithmetic
        s := add(shr(224, a), shr(224, b))
        s := add(s, add(and(shr(192, a), M), and(shr(192, b), M)))
        s := add(s, add(and(shr(160, a), M), and(shr(160, b), M)))
        s := add(s, add(and(shr(128, a), M), and(shr(128, b), M)))
        s := add(s, add(and(shr(96, a), M), and(shr(96, b), M)))
        s := add(s, add(and(shr(64, a), M), and(shr(64, b), M)))
        s := add(s, add(and(shr(32, a), M), and(shr(32, b), M)))
        s := add(s, add(and(a, M), and(b, M)))
        // Single reduction at end
        s := mod(s, R_MOD)
      }

      /**
       * @dev Applies INTERNAL MDS mixing to the first state element.
       *
       * @notice
       * This matrix is:
       * - Dense enough to ensure diffusion
       * - Structured to avoid full matrix multiplication
       * The full matrix is filled with 1s, and the diagonal is [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
       *
       *
       * Each limb is updated as:
       *   new_limb = sum + (constant × old_limb)
       */
      function matMulInternalInPlaceFirstHalf(a, sum) -> ma {
        let M := 0xFFFFFFFF
        ma := shl(224, addmod(sum, sub(R_MOD, mulmod(shr(224, a), 2, R_MOD)), R_MOD))
        ma := or(ma, shl(192, addmod(sum, and(shr(192, a), M), R_MOD)))
        {
          let x := and(shr(160, a), M)
          ma := or(ma, shl(160, addmod(sum, addmod(x, x, R_MOD), R_MOD)))
        }
        ma := or(ma, shl(128, addmod(sum, mulmod(and(shr(128, a), M), 1065353217, R_MOD), R_MOD)))
        ma := or(ma, shl(96, addmod(sum, mulmod(and(shr(96, a), M), 3, R_MOD), R_MOD)))
        ma := or(ma, shl(64, addmod(sum, mulmod(and(shr(64, a), M), 4, R_MOD), R_MOD)))
        ma := or(ma, shl(32, addmod(sum, mulmod(and(shr(32, a), M), 1065353216, R_MOD), R_MOD)))
        ma := or(ma, addmod(sum, sub(R_MOD, mulmod(and(a, M), 3, R_MOD)), R_MOD))
      }

      /**
       * @dev Internal MDS for second state element.
       * Diagonal: [-4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
       */
      function matMulInternalInPlaceSecondHalf(b, sum) -> mb {
        let M := 0xFFFFFFFF
        mb := shl(224, addmod(sum, sub(R_MOD, mulmod(shr(224, b), 4, R_MOD)), R_MOD))
        mb := or(mb, shl(192, addmod(sum, mulmod(and(shr(192, b), M), 2122383361, R_MOD), R_MOD)))
        mb := or(mb, shl(160, addmod(sum, mulmod(and(shr(160, b), M), 1864368129, R_MOD), R_MOD)))
        mb := or(mb, shl(128, addmod(sum, mulmod(and(shr(128, b), M), 2130706306, R_MOD), R_MOD)))
        mb := or(mb, shl(96, addmod(sum, mulmod(and(shr(96, b), M), 8323072, R_MOD), R_MOD)))
        mb := or(mb, shl(64, addmod(sum, mulmod(and(shr(64, b), M), 266338304, R_MOD), R_MOD)))
        mb := or(mb, shl(32, addmod(sum, mulmod(and(shr(32, b), M), 133169152, R_MOD), R_MOD)))
        mb := or(mb, addmod(sum, mulmod(and(b, M), 127, R_MOD), R_MOD))
      }

      /**
       * @dev Applies the FULL (external) Poseidon2 MDS matrix.
       *
       * In the following, M4 =
       * (2 3 1 1)
       * (1 2 3 1)
       * (1 1 2 3)
       * (3 1 1 2)
       * Steps:
       * 1. Apply 4×4 MDS to each half (matMulM4uint256, where the MDS is circ(2M4,M4,..,M4))
       * 2. Compute column sums across both state elements
       * 3. Redistribute column sums to all limbs
       *
       * This ensures complete diffusion across the entire state.
       */
      function matMulExternalInPlace(a, b) -> ra, rb {
        ra := matMulM4uint256Lazy(a)
        rb := matMulM4uint256Lazy(b)

        let t0, t1, t2, t3 := sumColumnsLazy(ra, rb)

        ra := matMulExternalInPlaceFirstHalf(ra, t0, t1, t2, t3)
        rb := matMulExternalInPlaceSecondHalf(rb, t0, t1, t2, t3)
      }

      /**
       * @dev Final redistribution step of EXTERNAL MDS for first state element.
       *
       * Here:
       * [t0, t1, t2, t3] = M4*a_hi+M4*a_lo+M4*b_hi+M4*b_lo (computed with matMulM4uint256 followed by sumColumns, see matMulExternalInPlace)
       * a = diag(M4, M4)*[a_hi, a_lo]
       *
       * This function completes the computation of circ(2M4,M4,M4,M4)*[a, b] by adding t0, t1, t2, t3] to a_hi and a_lo
       *
       */
      function matMulExternalInPlaceFirstHalf(a, t0, t1, t2, t3) -> ra {
        let M := 0xFFFFFFFF
        ra := shl(224, addmod(t0, shr(224, a), R_MOD))
        ra := or(ra, shl(192, addmod(t1, and(shr(192, a), M), R_MOD)))
        ra := or(ra, shl(160, addmod(t2, and(shr(160, a), M), R_MOD)))
        ra := or(ra, shl(128, addmod(t3, and(shr(128, a), M), R_MOD)))
        ra := or(ra, shl(96, addmod(t0, and(shr(96, a), M), R_MOD)))
        ra := or(ra, shl(64, addmod(t1, and(shr(64, a), M), R_MOD)))
        ra := or(ra, shl(32, addmod(t2, and(shr(32, a), M), R_MOD)))
        ra := or(ra, addmod(t3, and(a, M), R_MOD))
      }

      /**
       * @dev Final redistribution step of EXTERNAL MDS for second state element.
       *
       * Uses the same column sums but applies them to the second word.
       */
      function matMulExternalInPlaceSecondHalf(b, t0, t1, t2, t3) -> rb {
        let M := 0xFFFFFFFF
        rb := shl(224, addmod(t0, shr(224, b), R_MOD))
        rb := or(rb, shl(192, addmod(t1, and(shr(192, b), M), R_MOD)))
        rb := or(rb, shl(160, addmod(t2, and(shr(160, b), M), R_MOD)))
        rb := or(rb, shl(128, addmod(t3, and(shr(128, b), M), R_MOD)))
        rb := or(rb, shl(96, addmod(t0, and(shr(96, b), M), R_MOD)))
        rb := or(rb, shl(64, addmod(t1, and(shr(64, b), M), R_MOD)))
        rb := or(rb, shl(32, addmod(t2, and(shr(32, b), M), R_MOD)))
        rb := or(rb, addmod(t3, and(b, M), R_MOD))
      }

      /**
       * @dev Computes column sums for EXTERNAL MDS mixing with lazy reduction.
       * Each column sums 4 limbs < P, so sum < 4P < 2^33.
       *
       * Groups limbs into columns:
       *   column 0: limbs 0 + 4
       *   column 1: limbs 1 + 5
       *   column 2: limbs 2 + 6
       *   column 3: limbs 3 + 7
       *
       * This prepares shared values reused across both state elements,
       * reducing duplicated computation.
       */
      function sumColumnsLazy(a, b) -> t0, t1, t2, t3 {
        let M := 0xFFFFFFFF
        // t0 = a[0] + a[4] + b[0] + b[4], all < P, so sum < 4P
        t0 := mod(add(add(shr(224, a), and(shr(96, a), M)), add(shr(224, b), and(shr(96, b), M))), R_MOD)
        t1 := mod(
          add(add(and(shr(192, a), M), and(shr(64, a), M)), add(and(shr(192, b), M), and(shr(64, b), M))),
          R_MOD
        )
        t2 := mod(
          add(add(and(shr(160, a), M), and(shr(32, a), M)), add(and(shr(160, b), M), and(shr(32, b), M))),
          R_MOD
        )
        t3 := mod(add(add(and(shr(128, a), M), and(a, M)), add(and(shr(128, b), M), and(b, M))), R_MOD)
      }

      /**
       * @dev Applies the 4×4 MDS matrix twice to a packed uint256 with lazy reduction in matMulM4.
       *
       * Layout:
       *   uint256 = [v0 v1 v2 v3 | v4 v5 v6 v7]
       *
       * The function:
       * - Applies matMulM4 to the top 4 limbs
       * - Applies matMulM4 to the bottom 4 limbs
       *
       * This preserves Poseidon2’s structured MDS design while
       * keeping operations word-local and gas-efficient.
       */
      function matMulM4uint256Lazy(a) -> b {
        let M := 0xFFFFFFFF
        {
          let v0 := shr(224, a)
          let v1 := and(shr(192, a), M)
          let v2 := and(shr(160, a), M)
          let v3 := and(shr(128, a), M)
          let s0, s1, s2, s3 := matMulM4Lazy(v0, v1, v2, v3)
          b := or(shl(224, s0), shl(192, s1))
          b := or(b, shl(160, s2))
          b := or(b, shl(128, s3))
        }
        {
          let v0 := and(shr(96, a), M)
          let v1 := and(shr(64, a), M)
          let v2 := and(shr(32, a), M)
          let v3 := and(a, M)
          let s4, s5, s6, s7 := matMulM4Lazy(v0, v1, v2, v3)
          b := or(b, shl(96, s4))
          b := or(b, shl(64, s5))
          b := or(b, shl(32, s6))
          b := or(b, s7)
        }
      }

      /**
       * @dev 4x4 MDS matrix multiply with lazy reduction.
       * Inputs a,b,c,d < P. Intermediate sums < 7P < 2^34.
       * Uses regular add internally, mod only on outputs.
       * Saves: 11 addmod (88 gas) -> 7 add + 4 mod (41 gas) = 47 gas per call
       *
       * The MDS matrix is
       * (2 3 1 1)
       * (1 2 3 1)
       * (1 1 2 3)
       * (3 1 1 2)
       */
      function matMulM4Lazy(a, b, c, d) -> u, v, w, x {
        // All intermediate values fit in 256 bits
        let t01 := add(a, b) // < 2P
        let t23 := add(c, d) // < 2P
        let t0123 := add(t01, t23) // < 4P
        let t01123 := add(t0123, b) // < 5P
        let t01233 := add(t0123, d) // < 5P
        // Outputs need reduction for correct packing into 32-bit limbs
        x := mod(add(add(a, a), t01233), R_MOD) // 2a + t01233 < 7P
        v := mod(add(add(c, c), t01123), R_MOD) // 2c + t01123 < 7P
        u := mod(add(t01, t01123), R_MOD) // < 7P
        w := mod(add(t23, t01233), R_MOD) // < 7P
      }

      /**
       * @dev Add round key WITHOUT reduction - sbox will reduce via mulmod.
       * Since x[i] < P and k[i] < P, sum < 2P < 2^32, fits in 32-bit limb.
       * Saves: 8 addmod (64 gas) -> 8 add (24 gas) = 40 gas per call
       */
      function addRoundKeyUint256UnreducedAndSbox(x, k) -> rx {
        let M := 0xFFFFFFFF
        rx := shl(224, sboxSingleLimb(add(shr(224, x), shr(224, k))))
        rx := or(rx, shl(192, sboxSingleLimb(add(and(shr(192, x), M), and(shr(192, k), M)))))
        rx := or(rx, shl(160, sboxSingleLimb(add(and(shr(160, x), M), and(shr(160, k), M)))))
        rx := or(rx, shl(128, sboxSingleLimb(add(and(shr(128, x), M), and(shr(128, k), M)))))
        rx := or(rx, shl(96, sboxSingleLimb(add(and(shr(96, x), M), and(shr(96, k), M)))))
        rx := or(rx, shl(64, sboxSingleLimb(add(and(shr(64, x), M), and(shr(64, k), M)))))
        rx := or(rx, shl(32, sboxSingleLimb(add(and(shr(32, x), M), and(shr(32, k), M)))))
        rx := or(rx, sboxSingleLimb(add(and(x, M), and(k, M))))
      }

      /**
       * @dev Add round key WITH reduction - used for final output.
       */
      function addRoundKeyUint256Reduced(x, k) -> rx {
        let M := 0xFFFFFFFF
        rx := shl(224, addmod(shr(224, x), shr(224, k), R_MOD))
        rx := or(rx, shl(192, addmod(and(shr(192, x), M), and(shr(192, k), M), R_MOD)))
        rx := or(rx, shl(160, addmod(and(shr(160, x), M), and(shr(160, k), M), R_MOD)))
        rx := or(rx, shl(128, addmod(and(shr(128, x), M), and(shr(128, k), M), R_MOD)))
        rx := or(rx, shl(96, addmod(and(shr(96, x), M), and(shr(96, k), M), R_MOD)))
        rx := or(rx, shl(64, addmod(and(shr(64, x), M), and(shr(64, k), M), R_MOD)))
        rx := or(rx, shl(32, addmod(and(shr(32, x), M), and(shr(32, k), M), R_MOD)))
        rx := or(rx, addmod(and(x, M), and(k, M), R_MOD))
      }

      /**
       * @dev Add round key to first entry WITHOUT reduction.
       */
      function addRoundKeyFirstEntryUnreducedAndSbox(x, k) -> rx {
        let a0 := shr(224, x)
        let t0 := sboxSingleLimb(add(k, a0)) // < 2P, fits in 32 bits
        rx := sub(x, shl(224, a0))
        rx := add(rx, shl(224, t0))
      }

      /**
       * @dev S-box on a single limb. mulmod naturally reduces unreduced inputs.
       * Input limb can be < 2P (unreduced), output is always < P.
       */
      function sboxSingleLimb(x) -> rx {
        rx := mulmod(x, mulmod(x, x, R_MOD), R_MOD)
      }

      function revert_with_error(selector) {
        let ptr := mload(0x40)
        mstore(ptr, selector)
        revert(ptr, 4)
      }
    }
  }

  /**
   * @notice Pads a bytes32 input into a bytes array by splitting it into two 32-byte segments.
   * @dev Every two bytes are prepended with 2 zero bytes. E.g. 0xAAAABBBB -> 0x0000AAAA0000BBBB.
   * @param input The bytes32 input to be padded.
   * @return out A bytes array containing the two 32-byte segments.
   */
  function padBytes32(bytes32 input) external pure returns (bytes memory out) {
    assembly {
      out := mload(0x40)
      mstore(out, 0x40)

      let data := add(out, 0x20)
      let w := 0

      for {
        let i := 0
      } lt(i, 0x8) {
        i := add(i, 0x1)
      } {
        let v := and(shr(mul(sub(0xF, i), 0x10), input), 0xFFFF)
        w := or(w, shl(mul(sub(0x7, i), 0x20), v))
      }

      mstore(data, w)
      w := 0

      for {
        let i := 0x8
      } lt(i, 0x10) {
        i := add(i, 0x1)
      } {
        let v := and(shr(mul(sub(0xF, i), 0x10), input), 0xFFFF)
        w := or(w, shl(mul(sub(0xF, i), 0x20), v))
      }

      mstore(add(data, 0x20), w)
      mstore(0x40, add(data, 0x40))
    }
  }
}

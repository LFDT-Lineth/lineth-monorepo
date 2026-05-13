/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.maru

/**
 * Utility tool to compute the expected difficulty at a given time for Clique blocks.
 *
 * Clique blocks have:
 * - difficulty = blockNumber * 2 + 1
 * - block time = 2 seconds
 * - expected timestamp[i] = timestamp[i-1] + 2
 */
object DifficultyCalculator {
  /**
   * Computes the expected difficulty at the desired switch time.
   *
   * @param currentBlockNumber The current block number
   * @param currentTimestamp The timestamp of the current block
   * @param desiredSwitchTime The target timestamp for which we want to compute the difficulty
   * @return The expected block number and difficulty at the desired switch time
   */
  fun computeExpectedDifficulty(
    currentBlockNumber: Long,
    currentTimestamp: Long,
    desiredSwitchTime: Long,
  ): DifficultyResult {
    require(desiredSwitchTime >= currentTimestamp) {
      "Desired switch time ($desiredSwitchTime) must be >= current timestamp ($currentTimestamp)"
    }

    // Calculate how many seconds until the desired switch time
    val timeDifference = desiredSwitchTime - currentTimestamp

    // Calculate how many blocks will be produced (block time is 2 seconds)
    val blocksToAdd = timeDifference / 2

    // Calculate the expected block number at switch time
    val expectedBlockNumber = currentBlockNumber + blocksToAdd

    // Calculate the difficulty for that block: difficulty = blockNumber * 2 + 1
    val expectedDifficulty = expectedBlockNumber * 2 + 1

    println("=== Difficulty Calculator Debug ===")
    println("Current block: $currentBlockNumber")
    println("Current timestamp: $currentTimestamp")
    println("Desired switch time: $desiredSwitchTime")
    println("Time difference: $timeDifference seconds")
    println("Blocks to add: $blocksToAdd")
    println("Expected block number: $expectedBlockNumber")
    println("Expected difficulty: $expectedDifficulty")

    return DifficultyResult(expectedBlockNumber, expectedDifficulty, desiredSwitchTime)
  }

  data class DifficultyResult(
    val blockNumber: Long,
    val difficulty: Long,
    val timestamp: Long,
  ) {
    override fun toString(): String = "Block: $blockNumber, Difficulty: $difficulty, Timestamp: $timestamp"
  }

  @JvmStatic
  fun main(args: Array<String>) {
    if (args.size != 3) {
      println("Usage: DifficultyCalculator <currentBlockNumber> <currentTimestamp> <desiredSwitchTime>")
      println("Example: DifficultyCalculator 1000 1692000000 1692001000")
      return
    }

    try {
      val currentBlockNumber = args[0].toLong()
      val currentTimestamp = args[1].toLong()
      val desiredSwitchTime = args[2].toLong()

      val result = computeExpectedDifficulty(currentBlockNumber, currentTimestamp, desiredSwitchTime)

      println("Expected result:")
      println(result)
    } catch (e: NumberFormatException) {
      println("Error: All arguments must be valid numbers")
      println("Usage: DifficultyCalculator <currentBlockNumber> <currentTimestamp> <desiredSwitchTime>")
    } catch (e: IllegalArgumentException) {
      println("Error: ${e.message}")
    }
  }
}

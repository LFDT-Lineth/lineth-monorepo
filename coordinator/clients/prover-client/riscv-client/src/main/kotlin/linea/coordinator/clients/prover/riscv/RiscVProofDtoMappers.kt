package linea.coordinator.clients.prover.riscv

import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.L2ExecutionProofResponse
import linea.clients.RollupProofPublicInputs
import linea.clients.RollupProofResponse
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import kotlin.time.Instant

// ---------------------------------------------------------------------------------------------------------------------
// to/fromDomainObject helper functions between the RISC-V proof DTOs (RiscVProofDtos.kt) and their domain twins.
// ---------------------------------------------------------------------------------------------------------------------

internal fun L2ExecutionProofPublicInputsDto.toDomainObject(): L2ExecutionProofPublicInputs {
  return L2ExecutionProofPublicInputs(
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = endBlockTimestamp.toULong(),
    l2L1MessagesHash = l2L1MessagesHash.decodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toULong(),
    filteredAddressesHash = filteredAddressesHash.decodeHex(),
    txFromsHash = txFromsHash.decodeHex(),
  )
}

internal fun L2ExecutionProofPublicInputs.fromDomainObject(): L2ExecutionProofPublicInputsDto {
  return L2ExecutionProofPublicInputsDto(
    parentBlockHash = parentBlockHash.encodeHex(),
    endBlockHash = endBlockHash.encodeHex(),
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.toLong(),
    l2L1MessagesHash = l2L1MessagesHash.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    txFromsHash = txFromsHash.encodeHex(),
  )
}

/**
 * Maps the RISC-V 14-field PI tuple DTO onto its domain twin. Shared by the rollup and rollup-aggregation response
 * mappers since both emit the same tuple (rollup_spec §2.4). Field names and types are identical, so it is a straight
 * field copy.
 */
internal fun RollupProofPublicInputsDto.toDomainObject(): RollupProofPublicInputs {
  return RollupProofPublicInputs(
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
    l2L1BridgeTransactionTree = l2L1BridgeTransactionTree.decodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toULong(),
    filteredAddressesHash = filteredAddressesHash.decodeHex(),
    parentShnarf = parentShnarf.decodeHex(),
    endShnarf = endShnarf.decodeHex(),
  )
}

internal fun RollupProofPublicInputs.fromDomainObject(): RollupProofPublicInputsDto {
  return RollupProofPublicInputsDto(
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.epochSeconds,
    l2L1BridgeTransactionTree = l2L1BridgeTransactionTree.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    parentShnarf = parentShnarf.encodeHex(),
    endShnarf = endShnarf.encodeHex(),
  )
}

internal fun L2ExecutionProofResponse.fromDomainObject(): L2ExecutionProofDto {
  return L2ExecutionProofDto(
    proof = proof.encodeHex(),
    startBlockNumber = startBlockNumber.toLong(),
    endBlockNumber = endBlockNumber.toLong(),
    publicInputs = publicInputs.fromDomainObject(),
    l2L1Messages = l2L1Messages.map { it.encodeHex() },
    txFroms = txFroms.map { it.encodeHex() },
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
  )
}

internal fun RollupProofResponse.fromDomainObject(): RollupProofDto {
  return RollupProofDto(
    proof = proof.encodeHex(),
    startBlockNumber = startBlockNumber.toLong(),
    endBlockNumber = endBlockNumber.toLong(),
    publicInputs = publicInputs.fromDomainObject(),
    l2L1Roots = l2L1Roots.map { it.encodeHex() },
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
  )
}

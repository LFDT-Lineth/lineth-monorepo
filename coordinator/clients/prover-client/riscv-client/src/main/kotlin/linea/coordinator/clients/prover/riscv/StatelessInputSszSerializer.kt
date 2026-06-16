package linea.coordinator.clients.prover.riscv

import linea.clients.ChainConfig
import linea.clients.ExecutionPayload
import linea.clients.ExecutionRequests
import linea.clients.ExecutionWitness

fun interface StatelessInputSszSerializer {
  fun getStatelessInputSsz(
    executionPayload: ExecutionPayload,
    executionWitness: ExecutionWitness,
    chainConfig: ChainConfig,
    versionedHashes: List<ByteArray>,
    parentBeaconBlockRoot: ByteArray,
    executionRequests: ExecutionRequests,
    publicKey: List<ByteArray>,
  ): ByteArray
}

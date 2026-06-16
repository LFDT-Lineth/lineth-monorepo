package linea.coordinator.clients.prover.riscv

import tech.pegasys.teku.infrastructure.ssz.impl.SszContainerImpl
import tech.pegasys.teku.infrastructure.ssz.schema.SszContainerSchema
import tech.pegasys.teku.infrastructure.ssz.schema.SszListSchema
import tech.pegasys.teku.infrastructure.ssz.schema.SszPrimitiveSchemas
import tech.pegasys.teku.infrastructure.ssz.schema.SszSchema
import tech.pegasys.teku.infrastructure.ssz.schema.collections.SszByteListSchema
import tech.pegasys.teku.infrastructure.ssz.schema.collections.SszByteVectorSchema

/**
 * Shared teku SSZ schemas mirroring the Python `Ssz*` containers for the stateless input.
 *
 * Used by both [L2ExecutionStatelessInputSszSerializer] (domain types) and [L2ExecutionStatelessInputDtoSszSerializer]
 * (DTOs) so the two paths serialize with an identical schema. SSZ is positional, so the field order in each container
 * must match the Python `Container` field order exactly. Initialization order matters: leaf/element schemas must
 * precede the containers that reference them.
 */
internal object StatelessInputSszSchemas {
  // --- SSZ max-length constants (mirror the Python module) ---
  private const val MAX_EXTRA_DATA_BYTES = 32L
  private const val MAX_BYTES_PER_TRANSACTION = 1L shl 30
  private const val MAX_TRANSACTIONS_PER_PAYLOAD = 1L shl 20
  private const val MAX_WITHDRAWALS_PER_PAYLOAD = 1L shl 16
  private const val MAX_BLOB_COMMITMENTS_PER_BLOCK = 4096L
  private const val MAX_EXECUTION_REQUESTS = 16L
  private const val MAX_BYTES_PER_REQUEST = 1L shl 20
  private const val MAX_BLOCK_ACCESS_LIST_BYTES = 1L shl 24
  private const val MAX_WITNESS_NODES = 1L shl 20
  private const val MAX_WITNESS_CODES = 1L shl 16
  private const val MAX_WITNESS_HEADERS = 256L
  private const val MAX_BYTES_PER_WITNESS_NODE = 1L shl 20
  private const val MAX_BYTES_PER_CODE = 1L shl 24
  private const val MAX_BYTES_PER_HEADER = 1L shl 10
  private const val MAX_PUBLIC_KEYS = 1L shl 20
  private const val MAX_BYTES_PER_PUBLIC_KEY = 48L

  // --- fixed-size byte vectors ---
  val bytes32 = SszByteVectorSchema.create(32)
  val address = SszByteVectorSchema.create(20)
  val bloom = SszByteVectorSchema.create(256)

  // --- variable-size byte lists (leaf elements) ---
  val extraData = SszByteListSchema.create(MAX_EXTRA_DATA_BYTES)
  val transactionBytes = SszByteListSchema.create(MAX_BYTES_PER_TRANSACTION)
  val blockAccessList = SszByteListSchema.create(MAX_BLOCK_ACCESS_LIST_BYTES)
  val requestBytes = SszByteListSchema.create(MAX_BYTES_PER_REQUEST)
  val witnessNodeBytes = SszByteListSchema.create(MAX_BYTES_PER_WITNESS_NODE)
  val witnessCodeBytes = SszByteListSchema.create(MAX_BYTES_PER_CODE)
  val witnessHeaderBytes = SszByteListSchema.create(MAX_BYTES_PER_HEADER)
  val publicKeyBytes = SszByteListSchema.create(MAX_BYTES_PER_PUBLIC_KEY)

  // --- lists of leaf elements ---
  val transactions = SszListSchema.create(transactionBytes, MAX_TRANSACTIONS_PER_PAYLOAD)
  val versionedHashes = SszListSchema.create(bytes32, MAX_BLOB_COMMITMENTS_PER_BLOCK)
  val executionRequests = SszListSchema.create(requestBytes, MAX_EXECUTION_REQUESTS)
  val witnessState = SszListSchema.create(witnessNodeBytes, MAX_WITNESS_NODES)
  val witnessCodes = SszListSchema.create(witnessCodeBytes, MAX_WITNESS_CODES)
  val witnessHeaders = SszListSchema.create(witnessHeaderBytes, MAX_WITNESS_HEADERS)
  val publicKeys = SszListSchema.create(publicKeyBytes, MAX_PUBLIC_KEYS)

  // --- containers (order: dependencies first) ---
  val withdrawal = container(
    listOf(
      SszPrimitiveSchemas.UINT64_SCHEMA, // index
      SszPrimitiveSchemas.UINT64_SCHEMA, // validator_index
      address, // address
      SszPrimitiveSchemas.UINT256_SCHEMA, // amount
    ),
  )
  val withdrawals = SszListSchema.create(withdrawal, MAX_WITHDRAWALS_PER_PAYLOAD)

  val executionPayload = container(
    listOf(
      bytes32, // parent_hash
      address, // fee_recipient
      bytes32, // state_root
      bytes32, // receipts_root
      bloom, // logs_bloom
      bytes32, // prev_randao
      SszPrimitiveSchemas.UINT64_SCHEMA, // block_number
      SszPrimitiveSchemas.UINT64_SCHEMA, // gas_limit
      SszPrimitiveSchemas.UINT64_SCHEMA, // gas_used
      SszPrimitiveSchemas.UINT64_SCHEMA, // timestamp
      extraData, // extra_data
      SszPrimitiveSchemas.UINT256_SCHEMA, // base_fee_per_gas
      bytes32, // block_hash
      transactions, // transactions
      withdrawals, // withdrawals
      SszPrimitiveSchemas.UINT64_SCHEMA, // blob_gas_used
      SszPrimitiveSchemas.UINT64_SCHEMA, // excess_blob_gas
      blockAccessList, // block_access_list
    ),
  )

  val newPayloadRequest = container(
    listOf(
      executionPayload, // execution_payload
      versionedHashes, // versioned_hashes
      bytes32, // parent_beacon_block_root
      executionRequests, // execution_requests
    ),
  )

  val executionWitness = container(
    listOf(
      witnessState, // state
      witnessCodes, // codes
      witnessHeaders, // headers
    ),
  )

  val chainConfig = container(listOf(SszPrimitiveSchemas.UINT64_SCHEMA)) // chain_id

  val statelessInput = container(
    listOf(
      newPayloadRequest, // new_payload_request
      executionWitness, // witness
      chainConfig, // chain_config
      publicKeys, // public_keys
    ),
  )

  private fun container(fields: List<SszSchema<*>>): SszContainerSchema<SszContainerImpl> =
    SszContainerSchema.create(fields) { schema, backingNode -> SszContainerImpl(schema, backingNode) }
}

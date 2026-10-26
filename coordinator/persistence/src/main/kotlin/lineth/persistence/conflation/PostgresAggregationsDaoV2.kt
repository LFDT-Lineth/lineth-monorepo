package lineth.persistence.conflation

import io.vertx.sqlclient.SqlClient
import linea.clients.RollupAggregationProofResponseV1
import linea.domain.AggregationG
import lineth.coordinator.clients.prover.serialization.RollupAggregationProofResponseV1JsonResponse
import lineth.persistence.AggregationsDaoV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Instant

class PostgresAggregationsDaoV2(
  connection: SqlClient,
  clock: Clock = Clock.System,
) : PostgresAggregationsDaoG<RollupAggregationProofResponseV1>(connection, clock), AggregationsDaoV2 {

  override fun serializeProof(proof: RollupAggregationProofResponseV1?): String? {
    return proof?.let { RollupAggregationProofResponseV1JsonResponse.fromDomainObject(it).toJsonString() }
  }

  override fun deserializeProof(json: String): RollupAggregationProofResponseV1 {
    return RollupAggregationProofResponseV1JsonResponse.fromJsonString(json).toDomainObject()
  }

  override fun proofFinalTimestamp(proof: RollupAggregationProofResponseV1): Instant =
    proof.publicInputs.endBlockTimestamp

  override fun saveNewAggregation(aggregation: AggregationG<RollupAggregationProofResponseV1>): SafeFuture<Unit> {
    return saveAggregationRecord(aggregation)
  }
}

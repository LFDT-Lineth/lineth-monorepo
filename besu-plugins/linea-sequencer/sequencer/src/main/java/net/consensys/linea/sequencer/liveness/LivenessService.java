package net.consensys.linea.sequencer.liveness;

import java.util.Optional;
import net.consensys.linea.bundles.TransactionBundle;

public interface LivenessService {
  Optional<TransactionBundle> checkBlockTimestampAndBuildBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber);

  void updateUptimeMetrics(boolean isSucceeded, long blockTimestamp);
}

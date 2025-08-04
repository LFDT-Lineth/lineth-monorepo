package net.consensys.linea.sequencer.liveness;

import java.io.IOException;
import org.hyperledger.besu.ethereum.core.Transaction;

public interface LivenessTxBuilder {
  Transaction buildUptimeTransaction(boolean isUp, long timestamp, long nonce) throws IOException;
}

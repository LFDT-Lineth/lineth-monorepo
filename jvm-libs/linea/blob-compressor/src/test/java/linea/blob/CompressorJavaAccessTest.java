package linea.blob;

import org.junit.jupiter.api.Test;
import net.consensys.linea.blob.BlobCompressorVersion;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

public class CompressorJavaAccessTest {

  @Test
  public void shouldBeJavaCallerFriendly() {
    var compressor = GoBackedBlobCompressor.getInstance(
      BlobCompressorVersion.V1_0_1,
      102400);
    var compressedSize = compressor.compressedSize(new byte[1000]);

    assertThat(compressedSize).isBetween(0, 1000);
  }
}

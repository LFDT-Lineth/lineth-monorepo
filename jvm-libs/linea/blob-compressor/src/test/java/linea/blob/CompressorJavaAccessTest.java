package linea.blob;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import org.junit.jupiter.api.Test;

public class CompressorJavaAccessTest {

  @Test
  public void shouldBeJavaCallerFriendly() {
    var compressor = GoBackedBlobCompressor.getInstance(BlobCompressorVersion.V2, 102400);
    var compressedSize = compressor.compressedSize(new byte[1000]);

    assertThat(compressedSize).isBetween(0, 1000);
  }
}

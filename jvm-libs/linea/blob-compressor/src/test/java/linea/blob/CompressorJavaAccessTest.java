package linea.blob;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import org.junit.jupiter.api.Test;

public class CompressorJavaAccessTest {

  @Test
  public void legacyShouldBeJavaCallerFriendly() {
    var compressor = BlobCompressorFactory.getInstance(BlobCompressorVersion.V3, 102400);
    var compressedSize = compressor.compressedSize(new byte[1000]);

    assertThat(compressedSize).isBetween(0, 1000);
    assertThat(compressor.getVersion()).isEqualTo(BlobCompressorVersion.V3);
  }

  @Test
  public void shouldBeJavaCallerFriendly() {
    var compressor = BlobCompressorFactory.getInstance(BlobCompressorVersion.V4, 102400);
    var compressedSize = compressor.compressedSize(new byte[1000]);

    assertThat(compressedSize).isBetween(0, 1000);
    assertThat(compressor.getVersion()).isEqualTo(BlobCompressorVersion.V4);
  }
}

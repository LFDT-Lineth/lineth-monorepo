package net.consensys.linea.vertx

import io.restassured.RestAssured
import io.restassured.builder.RequestSpecBuilder
import io.restassured.http.ContentType
import io.restassured.module.kotlin.extensions.When
import io.restassured.specification.RequestSpecification
import io.vertx.core.Vertx
import net.consensys.linea.async.get
import org.assertj.core.api.Assertions.assertThat
import org.hamcrest.Matchers
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class ObservabilityServerTest {
  private lateinit var vertx: Vertx
  private lateinit var monitorRequestSpecification: RequestSpecification

  @BeforeEach
  fun beforeEach() {
    vertx = VertxFactory.createVertx()
    val port = runServerOnARandomPort(vertx)
    monitorRequestSpecification =
      RequestSpecBuilder()
        .setBaseUri("http://localhost:$port/")
        .build()
  }

  @AfterEach
  fun afterEach() {
    vertx.close().get()
  }

  @Test
  fun `when no port is defined it shall start server at random port`() {
    val observabilityServer = ObservabilityServer(
      ObservabilityServer.Config(
        applicationName = "test",
        port = 0, // random port assigned by underlying OS
      ),
    )
    val deploymentId = vertx.deployVerticle(observabilityServer).get()

    assertThat(observabilityServer.port).isGreaterThan(0)
    RestAssured.given()
      .spec(
        RequestSpecBuilder()
          .setBaseUri("http://localhost:${(observabilityServer.port)}/")
          .build(),
      )
      .When {
        get("/live")
      }
      .then()
      .statusCode(200)

    vertx.undeploy(deploymentId).get()
  }

  @Test
  fun exposesLiveEndpoint() {
    RestAssured.given()
      .spec(monitorRequestSpecification)
      .When {
        get("/live")
      }
      .then()
      .statusCode(200)
      .contentType(ContentType.JSON)
      .body("status", Matchers.equalTo("OK"))
  }

  @Test
  fun exposesHealthEndpoint() {
    RestAssured.given()
      .spec(monitorRequestSpecification)
      .When {
        get("/health")
      }
      .then()
      .statusCode(200)
      .contentType(ContentType.JSON)
      .body("status", Matchers.equalTo("UP"))
  }

  @Test
  fun exposesMetricsEndpoint() {
    // If no request is sent, no server metrics will appear
    RestAssured.given()
      .spec(monitorRequestSpecification)
      .accept(ContentType.JSON)
      .When {
        get("/metrics")
      }
      .then()
      .statusCode(200)

    RestAssured.given()
      .spec(monitorRequestSpecification)
      .accept(ContentType.JSON)
      .When {
        get("/metrics")
      }
      .also { assertThat(it.body.asString()).contains("vertx_http_server_response_bytes_bucket") }
      .then()
      .statusCode(200)
      .contentType(ContentType.TEXT)
  }

  private fun runServerOnARandomPort(vertx: Vertx): Int {
    val observabilityServer = ObservabilityServer(
      config = ObservabilityServer.Config(applicationName = "test"),
    )
    vertx.deployVerticle(observabilityServer).get()
    return observabilityServer.port
  }
}

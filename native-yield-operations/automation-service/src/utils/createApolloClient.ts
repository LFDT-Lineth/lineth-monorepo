import { ApolloClient, HttpLink, InMemoryCache, ApolloLink } from "@apollo/client";
import { SetContextLink } from "@apollo/client/link/context";
import { IOAuth2TokenClient } from "@consensys/linea-shared-utils";

/**
 * Creates an Apollo Client instance configured with OAuth2 authentication.
 * Sets up an HTTP link for GraphQL requests and an authentication link that injects
 * bearer tokens into request headers. The authentication link runs before the HTTP link
 * to ensure all requests are authenticated.
 *
 * @param {IOAuth2TokenClient} oAuth2TokenClient - The OAuth2 token client used to obtain bearer tokens for authentication.
 * @param {string} graphqlUri - The GraphQL endpoint URI.
 * @returns {ApolloClient} A configured Apollo Client instance with OAuth2 authentication and in-memory cache.
 */
// Could move to linea-shared-utils one day, if there is another project using @apollo/client
export function createApolloClient(oAuth2TokenClient: IOAuth2TokenClient, graphqlUri: string): ApolloClient {
  // --- create the base HTTP transport
  const httpLink = new HttpLink({
    uri: graphqlUri,
  });

  const asyncAuthLink = new SetContextLink(async (prevContext, operation) => {
    void operation;
    const bearerToken = await oAuth2TokenClient.getBearerToken();
    return {
      headers: {
        ...prevContext.headers,
        authorization: bearerToken,
      },
    };
  });
  // --- combine links so authLink runs before httpLink
  const client = new ApolloClient({
    link: ApolloLink.from([asyncAuthLink, httpLink]),
    cache: new InMemoryCache(),
  });
  return client;
}

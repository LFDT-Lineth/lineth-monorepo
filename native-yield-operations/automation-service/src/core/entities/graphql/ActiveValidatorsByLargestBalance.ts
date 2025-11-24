import { gql, TypedDocumentNode } from "@apollo/client";
import { ValidatorBalance } from "../ValidatorBalance.js";
// https://www.apollographql.com/docs/react/data/typescript#using-operation-types

/** Result shape returned by the server */
type ActiveValidatorsByLargestBalanceQuery = {
  allValidators: {
    nodes: Array<ValidatorBalance>;
  };
};

/** Variables you pass in */
type ActiveValidatorsByLargestBalanceQueryVariables = {
  first?: number; // optional since we set a default in the query
};

export const ALL_VALIDATORS_BY_LARGEST_BALANCE_QUERY: TypedDocumentNode<
  ActiveValidatorsByLargestBalanceQuery,
  ActiveValidatorsByLargestBalanceQueryVariables
> = gql`
  query AllValidatorsByLargestBalanceQuery($first: Int = 100) {
    allValidators(condition: { state: ACTIVE }, orderBy: EFFECTIVE_BALANCE_DESC, first: $first) {
      nodes {
        balance
        effectiveBalance
        publicKey
        validatorIndex
      }
    }
  }
`;

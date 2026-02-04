import { gql, TypedDocumentNode } from "@apollo/client";
import { ValidatorBalance } from "../Validator.js";
// https://www.apollographql.com/docs/react/data/typescript#using-operation-types

/** Result shape returned by the server */
type ActiveValidatorsByLargestBalanceQuery = {
  allHeadValidators: {
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
    allHeadValidators(filter: { state: { in: [DEPOSITED, ACTIVE] } }, orderBy: EXIT_EPOCH_ASC, first: $first) {
      nodes {
        balance
        effectiveBalance
        publicKey
        validatorIndex
        activationEpoch
      }
    }
  }
`;

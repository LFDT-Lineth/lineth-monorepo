import { gql, TypedDocumentNode } from "@apollo/client";
import { ExitedValidator } from "../Validator.js";
// https://www.apollographql.com/docs/react/data/typescript#using-operation-types

/** Result shape returned by the server */
type ExitedValidatorsQuery = {
  allHeadValidators: {
    nodes: Array<ExitedValidator>;
  };
};

/** Variables you pass in */
type ExitedValidatorsQueryVariables = {
  first?: number; // optional since we set a default in the query
};

export const EXITED_VALIDATORS_QUERY: TypedDocumentNode<ExitedValidatorsQuery, ExitedValidatorsQueryVariables> = gql`
  query ExitedValidatorsQuery($first: Int = 100) {
    allHeadValidators(filter: { state: { in: [EXITED] } }, orderBy: EXIT_EPOCH_ASC, first: $first) {
      nodes {
        balance
        publicKey
        validatorIndex
        slashed
        withdrawableEpoch
      }
    }
  }
`;

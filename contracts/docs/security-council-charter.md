# Linea Security Council Charter

This Charter outlines the structure and responsibilities of the Linea Security Council (LSC) and its members.

The Linea Security Council is composed of twelve members responsible for Linea's operational security and functionality. They operate a Gnosis Safe multisig wallet as signatories of 9/12 multisigs on the Ethereum and Linea blockchains. LSC members are tasked with creating, reviewing, and signing multisig transactions to upgrade and configure contracts on L1 Ethereum Mainnet and L2 Linea Mainnet, while protecting the chain and its users from any malicious change linked to the Security Council.

This charter sets forth the operations of the LSC and the responsibilities of its members. It provides operational guidelines to ensure the reliability, security, and efficiency of the Linea Security Council's activities and satisfy the requirements of Stages 0, 1, and 2 of the [L2Beat framework](https://l2beat.com/scaling/projects/linea#stage).

# Scope and responsibilities

LSC members are tasked with reviewing and signing multisig transactions to implement changes requested by Linea governance, on Ethereum Mainnet or Linea Mainnet. LSC members are required to carry out their responsibilities in a manner consistent with this Charter and pursuant to the decisions of Linea governance, while protecting the chain and its users from any malicious change linked to the Security Council. These operations include but are not limited to:

- Scheduling and Executing smart-contract upgrades and verifiers (based on OpenZeppelin Timelock controller)  
- Changing role designations (based on OZ and Zodiac roles)  
- Operationally configuring contracts  
- Signing unpause transactions (when an emergency pause was triggered)  
- Changing the signatories and threshold of the LSC

The contracts under the responsibility of the Security Council are (but not limited to):

1. The L1 and L2 multisigs themselves  
2. Finalization and bridge-related contracts:  
   1. L1 Rollup+MessageService, L2 MessageService  
   2. L1 and L2 TokenBridge  
   3. L1 Verifiers (indirectly) and the verification key of the zkEVM circuit  
3. Voyage and Surge-related contracts:  
   1. L2 LineaVoyageXP LXP  
   2. L2 LineaSurgeXP Token LXP-L  
4. ENS related contracts  
   1. L2 PublicResolver, NameWrapper, Registrars, etc.   
5. All TimelockController and ProxyAdmin related to the above contracts

The verification key of the zkEVM circuit connects the contract to the zkEVM itself.

# Signing process

1. ## Notice of transaction \- Linea Association

After Linea governance has determined that transactions should occur, the Linea Association shall send a notice of future transactions to be signed at least 48 hours before the actual signing request, with the context of the upcoming request and necessary inputs (audits, addresses on testnet, Github repository commits, …)

2. ## Transaction crafting \- Linea Association

The Linea Association will craft the transactions and send the transactions' descriptions (including Layer, Nonce, Action performed, and Verification). Linea team should try to give as much time as possible for the signature—48 hours minimum is recommended, more for complex operations.  
A Tenderly simulation may be shared (only) in case of a scheduled transaction to facilitate the review.

3. ## Transaction review \- Linea Security Council

Security Council members must carefully examine the transactions before signing, specifically the Event tab. Linea provides a static document with [diagrams of all the contracts under the Security Council's responsibility](./workflows/diagrams/Linea-Security-Council.png) and a static [Address book](./mainnet-address-book.csv) to be imported inside the Gnosis Safe App to facilitate the review of transactions.

The members of the Security Council are not expected to be able to review the smart contracts or the EVM circuits. Their review should ensure that:

* The transaction does what is effectively intended by the Linea governance, as documented by the Linea team  
* For smart-contract upgrades, the updates are audited, and the upgrade version is the same as the version audited. It includes all the recommended fixes and has already been deployed on the Sepolia and/or Linea Sepolia testnets.

# Additional requirements

## Eligibility

Members of the LSC should be selected by the Linea Association in accordance with the following criteria:

* Technical competency. Baseline proficiency with Linea Stack, experience with Safe infrastructure, and secure key management and signing standards.  
* Reputation. Known, trusted standalone individuals or entities that have demonstrated consistent alignment with the Ethereum and Linea ethos.  
* Geographic diversity. The number of participants who reside in any country should be less than 3\. The Linea Association will enforce this requirement as part of the eligibility screening process to avoid requiring participants to disclose their physical locations publicly.  
* Diversity of interests. No more than 1 participant is associated with a particular entity, or that entity’s employees or affiliates, with the exception of Linea Association or Consensys employees.  
* Alignment. Participants should not possess conflicts of interest that will regularly impact their ability to make impartial decisions in the performance of their role.

In addition, all participants must sign a standard contract obligating the member to comply with this Charter and all Linea governance instructions, among other things, which the Linea Association will implement at its procedural discretion.

Finally, standalone individuals or entities must publicly disclose their participation in the LSC when requested by the Linea Association.

## Availability of Signing Device

### Notification Requirement

Members must give the council at least 48 hours’ notice if they anticipate that their wallet, private key or signing device will not be in their possession for more than 48 hours. This notice should be communicated via the official LSC communication channel and should include the expected duration of unavailability.

### Immediate Loss Reporting

In case of a lost private key or suspicion of its compromise, members must inform the other LSC members and the Association immediately. Prompt disclosure allows for swift action to mitigate potential risks. The LSC can quickly replace the compromised address among the multisig signers, ensuring continued security.

## Liveness check

The Linea Association might require LSC members to undergo periodic liveness checks. In these checks, participants must sign a message proving they can access their keys within a set time limit.

## Communication and Response Times

LSC members are required to communicate with other members and monitor the communication channel designated by the Linea Association \- currently a Telegram Channel.

Each member must acknowledge all transaction requests made through the official LSC communication channel within 48 hours of receipt.

After acknowledging a request, LSC members must take necessary action or respond fully within 48 hours unless prior notification regarding the unavailability of the signing device has been communicated.

## Security requirements

### Dedicated Signing Device

Members must maintain their signing keys on a dedicated hardware device, such as a hardware wallet (e.g., Ledger, Trezor), used exclusively for LSC-related transactions. This practice minimizes the risk of unauthorized access and enhances the security of the signing process by isolating the signing keys from potentially compromised environments.

Private keys owned by the Linea Security Council members should either be:

- Keys managed by a single known individual or entity  
- A multisig owned by a few known individuals within the entity

### Continuous education and training

Members should actively participate in relevant training sessions, workshops, and courses to the extent set forth by the Linea Association. Additionally, members should review and familiarize themselves with process documents. This preparedness is crucial for effective and timely responses during critical situations.

## Conflict of Interest disclosure

Members must disclose to the Linea Association any conflicts of interest regarding their duties in LSC, including personal investments, affiliations, or relationships that might influence their decisions. This disclosure should occur per-transaction, allowing members to abstain from voting or signing when a conflict exists.

## Amendment

This charter can be amended at any time. Amendment proposals can be put forward by the Association or any LSC member. Amendments will be accepted, and this Charter will be revised and republished upon a majority vote of the LSC and the concurrence of the Association.     

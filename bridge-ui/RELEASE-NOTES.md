<a name="v2.0.0"></a>

# [v2.0.0] - 18 Mar 2025

# Feat: New bridge UI design

Description:
- New bridge UI v2 design

[Changes][v2.0.0]

<a name="v1.1.0"></a>

# [v1.1.0] - 2 Dec 2024

# Fix: New bridge UI colors

Description:
- New bridge UI colors

[Changes][v1.1.0]

<a name="v1.0.0"></a>

# [v1.0.0] - 18 Sep 2024

# Fix: New Bridge UI design

Description:
- New bridge UI design

[Changes][v1.0.0]


<a name="v0.6.5"></a>

# [v0.6.5] - 30 May 2024

# Fix: ETH and USDC history fetching issue

Description:
- Fix ETH and USDC history fetching issue

Technical implementation:
- [PR](https://github.com/Consensys/zkevm-monorepo/pull/3328)

[Changes][v0.6.5]

<a name="v0.6.4"></a>

# [v0.6.4] - 24 May 2024

# Feat: Add claim message as a recipient feature

Description:
- Add claim message as a recipient feature

Technical implementation:
- [PR](https://github.com/Consensys/zkevm-monorepo/pull/3132)

[Changes][v0.6.4]

<a name="v0.6.3"></a>

# [v0.6.3] - 15 May 2024

# Fix: Update shortcuts doc url and description

Description:
- Update shortcuts doc url
- Update shortcuts description

Technical implementation:
- [PR](https://github.com/Consensys/zkevm-monorepo/pull/3220)

[Changes][v0.6.3]

<a name="v0.6.2"></a>

# [v0.6.2] - 14 May 2024

# Fix: Enable shortcuts for easy bridging and update third party bridges link

Description:
- Update link for third party bridges on the landing page to https://linea.build/apps?types=bridge
- Enable shortcuts for easy bridging from L1 -> Linea

Technical implementation:
- [PR for third party bridges](https://github.com/Consensys/zkevm-monorepo/pull/3116)
- [PR to enable shortcuts](https://github.com/Consensys/zkevm-monorepo/pull/2883)

[Changes][v0.6.2]

<a name="v0.6.1"></a>

# [v0.6.1] - 9 Apr 2024

# Fix: ERC20 abi approve function return types to support USDT

Description:
- Fix ERC20 abi approve function return types to support USDT

Technical implementation:
[PR](https://github.com/Consensys/zkevm-monorepo/pull/2932)

[Changes][v0.6.1]

<a name="v0.6.0"></a>

# [v0.6.0] - 21 Mar 2024

# Feat: Switch to Linea Sepolia testnet

Description:
- Add support for Linea Sepolia testnet
- Remove support for Linea Goerli testnet

Technical implementation:
[PR](https://github.com/Consensys/zkevm-monorepo/pull/2735)

[Changes][v0.6.0]

<a name="v0.5.4"></a>

# [v0.5.4] - 14 Feb 2024

# Feat: Update Linea SDK to v0.2.1

Description:
To support the new claiming method, the Linea SDK has been updated to v0.2.1

[Changes][v0.5.4]

<a name="v0.5.3"></a>

# [v0.5.3] - 12 Jan 2024

# Feat: Switch from WalletConnect Modal to Web3Modal

Description:
Use Web3Modal as the connector's wrapper to connect to a wallet instead of WalletConnect Modal

# Chore: Use latest message service sdk

Description:
The message service sdk has been updated to take into consideration the changes regarding the way the messages are anchored from L2 to L1.

[Changes][v0.5.3]

<a name="v0.5.2"></a>

# [v0.5.2] - 15 Dec 2023

# Fix: MMM redirection infinite loop

Description:
Metamask connect for in the mobile app redirects the user indefinitely to the MM app

[Changes][v0.5.2]

<a name="v0.5.1"></a>

# [v0.5.1] - 08 Dec 2023

# Feat: Add GetFeeback button

Description:
Add the "Get feedback" button to the home page

# Fix: Referrer

Description:
Allow referrer when opening Metamask Portfolio

[Changes][v0.5.1]

<a name="v0.5.0"></a>

# [v0.5.0] - 01 Dec 2023

# Fix: Redirect user to Metamask mobile when on mobile

Description:
When browsing bridge UI on a mobile's browser, the user could not connect to Metamask.

# Fix: Transactions mix-up

Description:
Fix transactions being merged in storage for 2 different accounts when switching accounts.

# Fix: Wrong error message when switching network

Description:
An error message was being displayed after a transaction was successful if the user switched network while the transaction was loading.

# Improvement: Remove Default token list from user's storage

Description:
The user's custom tokens and default token list were merged in the user's local storage, there are now merged in the user's state and only the user's custom tokens are kept in storage. The default token list will be refresh every time the user refreshes.

<a name="v0.4.14"></a>

# [v0.4.14] - 23 Nov 2023

# Feature: Implement Linea's Official Token List

Description:
This feature replaces the default token configuration by incorporating the official token list from https://github.com/Consensys/linea-token-list.

Technical Implementation:
The Goerli and Mainnet JSON token lists are fetched from the GitHub repository and added to the user's storage, alongside any custom tokens added by the user.

# Feature: Introduce Landing Page for Bridge Selection

Description:
This update enhances the Linea Bridge user interface by directing users to the MM Bridge in their Portfolio.

Technical Implementation:
The marketing team has implemented this feature, adding a user interface context where the bridge options are displayed.

# Fix: Enable Addition of Tokens with Identical Addresses on Both Layers

Description:
Previously, users encountered an issue when attempting to add a token that shared the same address on both layers, making it impossible to add the token.

Technical Implementation:
Now, when searching for a custom token input by the user, the system will prioritize the current layer to which the user is connected.

# Fix: Address Issue of Null Balance Display for Certain Tokens

Description:
There was an issue where tokens, automatically added by fetching the user's transactions, sometimes lacked the token's address for one of the layers, resulting in a null balance display for that layer.

Technical Implementation:
Now, when fetching a custom token, the user interface will always search for the token's address on both layers if it is unknown.

# Fix: Correct Execution Fee Error Message

Description:
Previously, when a user connected to layer A (with 0 ETH) attempted to initiate a bridge transaction from layer B (where they had some ETH), they received an error message stating insufficient funds for execution fees, even though they had sufficient funds.

Technical Implementation:
The system now fetches the user's balance from the chain where the user intends to initiate the bridge, rather than the one connected to in their wallet.

<a name="v0.4.6"></a>

# [v0.4.6] - 14 Sep 2023

- remove Max button for ETH

<a name="v0.4.5"></a>

# [v0.4.5] - 14 Sep 2023

- add unit tests
- fix Max bug

<a name="v0.4.1"></a>

# [v0.4.1] - 21 Aug 2023

- Add tutorial video in the first popup
- Add Tutorial link in the footer

<a name="v0.4.0"></a>

# [v0.4.0] - 16 Aug 2023

# Feat: Display error message when exceeding daily withdrawal limit

Description:
There is a 1000 ETH daily withdrawal limit from Linea to L1 that triggers an error that needed to be displayed to the user.

Fix:
Display a toast error message.

Technical implementation:
Added an error handler for errors coming from viem when simulating or executing contract calls.

# Feat: Display error message when trying to bridge a reserved token

Description:
Some tokens are reserved and not bridgable, an error message needed to be added.

Fix:
Display a toast error message.

# Fix: Typo in automatic claiming tooltip

Description:
Changed text to :
"Automatic bridging: this fee is used to reimburse gas fees paid by the postman to execute for you the transaction on your behalf on the other chain. If gas fees are lower than the execution fees, the remaining amount will be reimbursed to the recipient address on the other chain."
# Feat: Change wording of clear history button

Description:
Changed "clear history" button to "Reload history"

# Fix: Max button issue

Description:
When clicking "max" and then clicking "start bridging," the page does not respond at all.

Fix:
Disable bridge button when amount + fees > user balance

Technical implementation:
Added fees in the form state to be retrieved in the amount checks.

# Fix: Claim button stays active while a claiming transaction is ongoing

Description:
Issue occurs when the ‘Claim Funds’ button is clicked and while the tx is processing, the user clicks again and another popup is displayed showing pending transaction incomplete.

Fix:
Disable claim button when a claim transaction is already ongoing for the same message hash

# Fix: Token list balances not updated properly

Description:
When claiming funds, the token list balance doesn't update but the balance on the homepage updates.

Technical implementation:
Used the same useBalance hook from wagmi being used in the form page for the token list balances.

# Feat: Add wstETH token

Description:
Add a new token to the default token list: wstETH

<a name="v0.3.14"></a>

# [v0.3.14] - 09 Aug 2023

### Show new tokens in the config

### Technical Implementation

- Modified token list in config
- Fix storing new token added to the config list

<a name="v0.3.13"></a>

# [v0.3.13] - 05 Aug 2023

### Support non standard ERC20 tokens

- Add support for non standard erc20 approve method (ex: USDT)

### Technical Implementation

- Updated abi used to call the method approve

<a name="v0.3.12"></a>

# [v0.3.12] - 04 Aug 2023

Prod release version

## Technical implementation

Just a bump version on `package.json`

<a name="v0.3.11"></a>

# [v0.3.11] - 04 Aug 2023

# Bug: Token with address 0x000000

Description:
Some Tokens on the other layer have a zero address, so it links to the explorer with zero address.

Fix:
No link when token has zero address.

Technical implementation:
compare to const ADDRESS_ZERO = "0x0000000000000000000000000000000000000000";
didn't found the equivalent to AddressZero (ethers) with wagmi/viem

# Feat: Simplify message

Description:
While waiting for bridging, the message was: `Waiting for the transaction to reach Linea Goerli Testnet`
Now it is: `Please wait, your funds are being bridged`

# Feat: History item UI changes

Description:

- display amount, from token, to token, to address on the same line
- add from layer transaction address + explorer link
- add an explorer link to destination address

# Bug: wrong selected token when switch network

Description:
Does not keep the right selected token when switching network

Fix:
Token is reseting when switching network

Technical implementation:
The reset was done on the switch network in the form, but not the header. New method `resetToken` in the chain context, called in both place.

<a name="v0.3.9"></a>

# [v0.3.9] - 03 Aug 2023

# Bug: wbtc decimals

## Description:

WBTC decimals were 18, but is it 8

## Fix:

Change decimals to 8

<a name="v0.3.8"></a>

# [v0.3.8] - 03 Aug 2023

# Bug: Explorer

## Description

For mainnet explorer was blockscout instead of Etherscan

## Fix

Replace the link with Etherscan

## Technical implementation

in bridge-ui/src/customChains/index.ts, change blockExplorers with "https://lineascan.build/"

# Feat: USDC

## Description

Add USDC smart contracts for Mainnet

## Technical implementation

{
"mainnet": {
"L1MessageService": "0xd19d4B5d358258f05D7B411E21A1460D11B0876F",
"FiatTokenV2_1": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
"L1USDCBridge": "0x504A330327A089d8364C4ab3811Ee26976d388ce"
},
"linea": {
"L2MessageService": "0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec",
"FiatTokenV2_1": "0x176211869cA2b568f2A7D4EE941E073a821EE1ff",
"L2USDCBridge": "0xA2Ee6Fce4ACB62D95448729cDb781e3BEb62504A"
}
}

# Feat: update packages

## Description

Update the project packages dependencies.

## Technical implementation

run npm outdated and update accordingly

<a name="v0.3.7"></a>

# [v0.3.7] - 03 Aug 2023

# Feature

## Description

Add the Mainnet token bridge addresses in config
L1: 0x051F1D88f0aF5763fB888eC4378b4D8B29ea3319
L2: 0x353012dc4a9A6cF55c941bADC267f82004A8ceB9

<a name="v0.3.6"></a>

# [v0.3.6] - 03 Aug 2023

# Feature:

## Description:

The history is refreshing once every 2 blocks. So:

when a bridge transaction is successful, it can take from 1 to 24 seconds to appear in the history.
when clearing the history, it can take from 1 to 24 seconds to start reconstructing
This feature is refreshing the history just after a successful transaction or clearing history, so the delay is minimized.

## Technical implementation:

fetchNext() is called after clearing and when a transaction is onSuccess()

# Feature:

## Description:

Readme technical documentation for deployment. Explains how to deploy the Bruidge ui on prod.

# Feature:

## Description:

When a user send a manual transaction, the transaction is loading, until displaying the loading button.
Not displaying a Claim button directly can lead to confusion for users.
The goal of the feature is to add a disabled Claim button for manual transaction while it is processing, to display to the user the action will be available soon.

## Technical implementation:

We check the message. if (message.fee === "0") { means it is manual so we display a Disabled button while the transaction is processing.

<a name="v0.3.5"></a>

# [v0.3.5] - 02 Aug 2023

**Bug:**
Little regression on 0.3.4, when adding a new token address, it was not possible to store it.

**Fix:**
A condition has been modified, it is possible to add new token on your list.

**Technical implementation:**
There was a wrong condition, we replaced `||` with `&&`

<a name="v0.3.4"></a>

# [v0.3.4] - 02 Aug 2023

# Feature

Display transactions even though the token involved is not in the user's token list
Fetches the tokens that are not in the user's token list from the transaction history and add them in the list automatically
Triggers a transaction history's reload when clicking on "Clear history"
Example of token:
UNI on goerli: 0x41E5E6045f91B61AACC99edca0967D518fB44CFB

# Bug

Bug: when you have a transaction pending, button is loading. If you change account the button is still loading waiting for the success of the other account transaction. It should reset the button state for the new account.

Fix: when you have an approval transaction or bridge transaction loading, and you switch account, the button goes back to not loading.

Technical implementation:
adding a useEffect on address, so everytime the address is changing, it reset the hash used by useWaitForTransaction, and it stops loading.

<a name="v0.3.3.1"></a>

# [v0.3.3] - 01 Aug 2023

<a name="v0.3.3"></a>

# [v0.3.3] - 01 Aug 2023

# High Level Description:

## Wait one minute before displaying claim button on L1 -> L2 bridge transactions

**Bug:** for a few seconds it shows the Claim button for automatic bridge.

When a bridge transaction has just been anchored on the other layer, there is a more or less one minute state where the transaction is CLAIMABLE even for automatic.

The goal of this feature, is to wait 5 blocks before showing the claim button. It will prevent user that used automatic claim, to temporary see the claim button appearing.

## Testing

Hard to be tested for non developers. Users should not complain anymore about seeing the claim button on Automatic Transactions.

# Technical Description:

When the transactions are retrieved, and the messages from SDK have been set.

Get the last 5 blocks of LINEA, and get L1L2MessageHashesAddedToInbox events. It returns the messageHash that have just been anchored.
If these messageHashes are also in the user transaction list, there transactions should not be claimable yet.

## Testing

Only dev can test easily, in useFetchAnchoringEvents
Replace BLOCK_LIMIT with
const BLOCK_LIMIT = BigInt(5000);
and L1 L2 Transactions should go back to loading

<a name="v0.3.2.1"></a>

# [v0.3.2] - 01 Aug 2023

<a name="v0.3.2"></a>

# [v0.3.2] - 01 Aug 2023

- New history system for ETH, ERC20 and USDC
- Retrieve the bridged token address

<a name="v0.1.17"></a>

# [v0.1.17] - 12 Jul 2023

Fix:

- Form resets when switching network
- Popup message "Token Bridging failed"
- Blue picto for testnet

<a name="v0.1.14"></a>

# [v0.1.14] - 12 Jul 2023

- Linea Mainnet alpha release

<a name="v0.1.13"></a>

# [v0.1.13] - 12 Jul 2023

- Mainnet alpha release

<a name="v0.1.12"></a>

# [v0.1.12] - 11 Jul 2023

<a name="v0.1.11"></a>

# [v0.1.11] - 11 Jul 2023

<a name="v0.0.2"></a>

# [v0.0.2] - 27 Jun 2023

<a name="v0.0.1"></a>

# [v0.0.1] - 27 Jun 2023

# Contributing

## Introduction

Thank you for your interest in contributing to Linea's zkEVM monorepo. This document provides guidelines and instructions on how to contribute effectively. We use trunk-based development, and our main languages are Kotlin, Golang, Solidity, and TypeScript. Our releases use GitOps, ArgoCD, Helm, and Kubernetes.

## Submit a Contribution

## Prerequisites

Before contributing, please ensure you're familiar with:

- Our [development guidelines](development-guidelines.md) on how to write code, tests, and documentation.
- Our [code of conduct](code-of-conduct.md).

### Create an issue

* Check beforehand that the issue you want to raise isn't present (even with other keywords) to avoid creating duplicates.
* Use an issue template. Using the correct template will aid other contributors in responding to your issue. Use `Feature request`, `Bug report` or `Operational Task`, depending on the context.
* Follow any directions described in the issue template itself. Be descriptive with the issue you are raising.
* Don't forget to highlight any impact on:
  * Internal changes: features that impact `Metrics`, `Runbooks`, `Operations`
  * External changes: features that impact `Documentation`, `Support`, `Community`, `Release notes`
* Assign appropriate labels.
* Be selective when assigning issues using /assign @<username> or /cc @<username>. Your issue will be triaged more effectively by applying correct labels over assigning more people to the issue.
* Leave issues in `Backlog`, prioritization in further columns are performed as part of sprint preparation.
* Issues should be closed only by the original author of the ticket after making sure it's fully completed and can be closed.

### Respond to an issue

* When tackling an issue, comment on it letting others know you are working on it to avoid duplicate work.
* When you have resolved something, comment on the issue letting people know before closing it.
* Include references to other PRs or issues (or any accessible materials), example: "ref: #1234". It is useful to identify that related work has been addressed somewhere else.

### Development process

We follow a [trunk-based](https://trunkbaseddevelopment.com/) development process. This means all developers work on a single branch, 'main', and features are developed in short-lived feature branches. Issues are your mandate, ensure you have an up-to-date issue when working on a feature or bug fix by editing the description and status. Once you start working on an issue, you become the owner of it, you are responsible to make it progress through the different steps and pass any blockers. Please escalate any problem you cannot overcome to the Linea team. Proper testing and documentation are always part of the definition of done.

1. **Create a new branch**: For each new feature or bug fix, create a new branch. Branches should be named descriptively, following the pattern `type/issue#-short-description`, e.g., `feature/123-add-login-button` or `bugfix/456-fix-login-error`.

1. **Write your code**: Develop your feature or fix the bug in your branch. Make sure your code is clean, well-commented, and adheres to our coding standards.
<!--- TODO: Section on coding standards -->

1. **Test your changes**: Write appropriate tests for your changes and ensure all tests pass before submitting a pull request. You are responsible for implementing robust but short-lived automated tests to avoid feature breaking because of a lack of automated test, and to avoid CI growing to an infinite time.
<!--- TODO: Section on unit & integration testing -->

1. **Commit your changes**: Commit your changes regularly and write clear, concise commit messages that explain your changes. Follow the pattern `type(issue#): short description of change`, e.g., `fix(456): corrected login error`.

1. **Push your changes**: Push your changes to your forked repository on GitHub.

1. **Submit a pull request**: Create a new pull request from your branch to the main repository's trunk/main branch. Include a detailed description of your changes and link the related issue number, in the template created.

1. **CI**: Ensure all elements in the CI are passing in a reliable fashion, don't retry to fix a flaky test. Ensure quality gates (tests, coverage, static analysis, and security analysis) are passing.

## Code reviews

Once you submit a pull request, it will be reviewed by the maintainers. They may ask for changes or improvements. Please be patient and responsive to their feedback.

## Release process

We use GitOps, ArgoCD, Helm, and Kubernetes for our release process. When changes are merged into the main branch, they can be deployed to a testnet environment using ArgoCD and Helm. After successful testing in the testnet environment, the changes can be promoted to the mainnet environment.

Contributors are responsible for pushing their changes to testnet and mainnet, and monitoring the impact of their changes.

The Release Manager is responsible for the technical soundness of each release. This includes:
* Ensuring that any code deployed to any environment (including Devnet) has corresponding E2E test coverage. If blockers prevent E2E coverage, they must be flagged and tracked before deployment proceeds.
* Reviewing each PR. Will it break something? Were necessary integration and regression tests done? What are dependencies? Are we following the correct order/timeline for testnet/mainnet?
* Ensuring communication has been shared with the relevant channels (Ops, SRE, support, community & documentation). Both with internal and external stakeholders (partners, community). This doesn't have to be done directly by the release manager, however the release manager is responsible for ensuring it's done.
* Ensuring people are available to react in case the release has an issue (engineer, SRE).
* Controlling that proper monitoring has been done by engineers when releasing.
* Make sure the release notes are accurate and up to date.
* Be the first contact point for operations/SRE if something happens on the cluster.
* Do a proper handover to the next release manager.

### Release types

* Hotfix:
  * Content: **critical** fixes that should be deployed ASAP. Fixes that are not critical will be part of the normal release calendar.
  * Frequency: ad hoc, anytime.
* Continuous release:
  * Content: Intermediary deliverables of major releases and non critical fixes.
  * Frequency: Every week. Internal developers are encouraged to release as frequently as possible their features and break them down using minor releases.

Configuration changes that are part of ops are not considered releases. This includes, for example, changing the # of provers or updating price thresholds. These configurations changes can be performed at any time.

### Standard deployment timeline

> External contributor's PRs will be added to the standard deployment flow and supported by a Consensys engineer.

* Consensys engineers share their PRs with the release manager weekly and the details are added to the Release Notes.
* The release manager greenlights the release, proposes a day and time for deploying the release to the engineer. The release manager ensures communication has been shared with internal stakeholders (SRE) as well as external stakeholders when relevant (external contributor/community/partners).
* The deployment on testnet is performed with the help of DevOps and SRE teams.
* The deployment on mainnet is performed following the exact same process as testnet, after sufficient time to be confident the release can be promoted to mainnet.

### Release to testnet
We release on a weekly basis. The standard internal contributor's day-by-day process is as follows:
1. **Create a testnet PR**: Create a new pull request to deploy the changes, once your PR has been merged. Only release changes that were merged in `Consensys/linea-monorepo/main`. Include a detailed description of your changes and link the related issue number.
2. **Prepare deployment with Release Manager**: Contributor reaches out to the Release Manager with the PR to prepare its inclusion in the next batch of releases. Use the next Testnet release, and add a tentative deployment to Mainnet at least 1 week after.
3. **Open a maintenance window for releases having an impact on user experience**: Internal contributors, reach out to NOC to share the expected maintenance window so it can be added to the [status page](https://linea.statuspage.io/). Maintenance windows can be requested by adding a ticket to the NOC board.
4. **Merge PR**: Contributor releases changes and follows instructions on the Testnet PR template. DevOps and SRE teams should be involved in the deployment activities. Major releases should include a detailed release plan that also includes a study of risks, mitigations, and revert plans.

### Release to mainnet

The deployment on mainnet is performed following the exact same process as testnet after sufficient time and testing to be confident the release can be promoted to mainnet.

## Conclusion

Your contributions are greatly appreciated. By following these guidelines, we can ensure a smooth and effective collaboration. Thank you for your contribution!

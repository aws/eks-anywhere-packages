# Curated Packages End-To-End Testing Plan

This document summarizes the current state and future plans for end-to-end (E2E) testing in the EKS-A Curated Packages (CP) project.

## The Present

End-to-end tests can be written to exercise CP workflows. Several parameters can be selected to control the environment (cluster) in which a test runs. These include: EKS-A provider, cluster node OS, and Kubernetes (K8s) version. For examples, see [TestCPackagesHarborInstallSimpleFlow](https://github.com/aws/eks-anywhere/blob/de84797d702028e15deb3363bbe222489f00229d/test/e2e/harbor_test.go) and [TestCPackagesVSphereKubernetes121SimpleFlow](https://github.com/aws/eks-anywhere/blob/de84797d702028e15deb3363bbe222489f00229d/test/e2e/curated_packages_test.go#L102).

Tests have been written that cover the installation and running of the CP packages controller and the [hello-eks-anywhere test application](https://github.com/aws/eks-anywhere-build-tooling/blob/61628f1669044a39ff561385c7032e3b6c7d12ed/projects/aws-containers/hello-eks-anywhere/conf/hello.sh) across the three dimensions mentioned above (EKS-A provider, node OS, and K8s version). Additional tests have been written to exercise Harbor and MetalLB package installations. The results of these tests are reported in the appropriate EKS-A communications channels.

*But the existing tests are not nearly enough to provide confidence that our packages are bug-free.* The current CP E2E framework does not (without significant effort by the test writer) run the existing tests against the versions of their artifacts that matter most[^1]. Nor do they run them for each submitted pull request (PR). These things are not the fault of the tests themselves, nor the tests' writers, but rather they've exposed the limitations of the current CP E2E framework.

[^1]: Early E2E tests were run against production artifacts, generally because they were the only reliably accessible artifacts at the time.

## The End-Goal

To be able to provide confidence that the CP offerings are working across our supported EKS-A versions, EKS-A providers, cluster node OSes, and K8s versions, the current framework must be improved in the following ways:

+ **Artifact version control:**
  Tests should be run against an arbitrary versions of artifacts, defaulting to their latest versions.

+ **Timing:**
  E2E tests should be run on every revision of an "ok to test" PR. This may present some challenges for the current E2E test architecture, as it might require communication between separate AWS pipelines, or failing that, custom code that can communicate with other AWS resources to identify and list available artifacts built in other pipelines.

+ **Platform configuration:**
  A test should be able to exclude specific EKS-A versions, EKS-A providers, cluster node OSes, or K8s versions with which they are incompatible or unsupported. A test will be run on every permutation by default.

+ **Configuration review & control:**
  The test dimensions should be easily viewed, verified, and edited by humans (or reasonable facsimiles thereof). This provides a means of quickly excluding tests categorically for reasons such as hardware outage.

Additionally, it is foreseen that the number of E2E tests that need to be run for each PR will quickly grow, and so it is recommended that these performance goals also be considered:

+ **Reusable clusters:** The amount of time required to create and destroy clusters before and after each test will quickly become overwhelming with reasonable amounts of hardware.

## The Path Forward

### Short Term

#### Test Against Current Branch

The current tests will be modified to run against the latest versions of artifacts. This will be accomplished by removing the existing hardcoded registry links from the existing tests, and implementing a means by which URLs to test-run-specific artifacts will be used. These up-to-date artifacts will be used (at a minimum) when installing the EKS-A Curated Packages controller during cluster creation, as well as when said controller acquires its active bundle. The newly-acquired active bundle will likewise contain links to newly-built test artifacts, such that the end-to-end tests will be run against artifacts built using the current git commits under test, and not the hard-coded development or production registries, as is the case now.

Test runs should be accomplished using development ECR repositories (i.e. public ECR), but if or where that isn't the case, credentials for accessing a private ECR would have to be acquired, which could prove a technical risk, as to my knowledge, this isn't presently done.

#### Investigate Daily E2E Run

Running end-to-end tests more frequently is another priority of the short term plan. An investigation into the feasibility of adding a periodic run of the end-to-end tests will be performed. If a periodic end-to-end run is estimated to be within a week's work, then it will be undertaken. The intention is to be notified of end-to-end test failures before a release branch is created, when deadlines are already set and there are possibly many time-sensitive tasks already waiting to be completed.

### Long Term

Following the short term goals, several broader, more in-depth changes will help our end-to-end tests further raise our confidence. What follows is a brief summary of the larger components or actions required, with a more detailed design after the short term priorities have been satisfied.

#### Test Manifests

A list of each artifact produced, along with identifiers and URLs to acquire each artifact for testing. This manifest will serve as one input to a new testing framework that will be able to ensure that the desired combinations of providers, host OSes, and any other test dimensions deemed appropriate are run.

This manifest will be generated in response to a pull request change or commit to selected branches (main, release-x, etc) of the eks-anywhere-packages, eks-anywhere, and eks-anywhere-build-tooling repositories (the last of which possibly limited to changes affecting a curated packages project), and will in turn be used by the testing framework.

#### Increased Frequency Of E2E Test Runs

If feasible, end-to-end tests will be run on each (ok-to-test) pull request on the eks-anywhere-packages repository. If that isn't feasible, then hourly or semi-hourly test runs will help catch problems sooner.

Aside: The quality of curated packages relies upon the quality of our end-to-end tests. Correspondingly, the more often we run them, the faster we learn of, and can fix, problems. If we're not able to run the full battery of tests for each PR, then it is strongly recommended that we do whatever is in our power to be able to do so.

#### Test Configuration Control

A means by which tests can be disabled or enabled based upon their properties, requirements, or current statuses. In plainer words, a way to write a test that doesn't run for a specific provider, or to temporarily disable tests for specific host OS. This input, along with the test manifest are envisioned as being the bulk of the input to a given CP E2E test run.

#### End-To-End Test Documentation

Documentation will be generated that, in addition to the forthcoming design document, will aid developers and other contributors understand the end-to-end test framework, specifically how to write a new end-to-end test, how to run an end-to-end test outside of the build pipelines, how to be notified of failures, and how to debug test failures.

A stretch goal could be to add documentation-style tests in the spirit of [Rust's documentation tests](https://doc.rust-lang.org/rustdoc/write-documentation/documentation-tests.html) or [go test examples](https://pkg.go.dev/testing#hdr-Examples).

#### Independent Build Tooling and Testing

In order for new CP end-to-end tests to be as straightforward as possible, and to run with the desired frequency goals, it is necessary to separate our build-tooling processes from those currently existing in the eks-anywhere and eks-anywhere-build-tooling repositories. The current processes produce significant developer friction when writing tests as well as running tests outside of the build pipelines. The existing system's builds are not as finely-grained as is required to meet the test-run-frequency goals defined previously in this document. Lastly, because the concerns of CP tests have little overlap with those of the EKS-A CLI, it is deemed appropriate that these distinctions in build and testing frameworks would allow each project to move more freely in directions that serves their individual goals best, while any infrastruture or utilities common to both could live in a neutral repository and shared by each.

There will likely be some small number of the current CP E2E tests, particularly those concerned with verifying that a CLI installation of the EKS-A CP controller is successful, that could remain in the eks-anywhere repository. The existing Prow infrastructure will be used, and reporting will be delivered in the same methods to minimize disruption to other systems, habits, or customs previously developed.

[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.CS.1] Implement automated digital attestation signing

**Category:** RECOMMENDED

Digital attestations serve as verifiable evidence that software components were built, tested, and conform to organizational standards within a controlled environment. Signatures associated with each attestation can be verified to ensure that the component has not been tampered with and originated from a trusted source. Generating attestations throughout the development lifecycle provides a method of ensuring software quality, origin, and authenticity.

Embed automated tools into the deployment pipeline to produce digital attestations. Create an attestation for each action you want to create proof for, such as a test being run, software being packaged, or even manual approval acceptance steps. Sign these attestations using symmetric or asymmetric keys. Follow metadata frameworks such as [in-toto](https://in-toto.io/) for best practices for formatting attestations to include metadata about the software, the build environment, and the authoring party. Store attestations either with build artifacts in a repository or within governance tools for deeper analysis.

**Related information:**

* [Software attestations](https://slsa.dev/attestation-model)


[Document Conventions](/general/latest/gr/docconventions.html)

Indicators for cryptographic signing

\[DL.CS.2] Sign code artifacts after each build

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
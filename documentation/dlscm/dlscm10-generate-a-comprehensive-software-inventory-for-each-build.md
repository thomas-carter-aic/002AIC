[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.SCM.10] Generate a comprehensive software inventory for each build

**Category:** RECOMMENDED

Maintain a comprehensive inventory of the components and dependencies that make up your software assists with identifying vulnerabilities and managing risks. This inventory, often taking the form of a [Software Bill of Materials (SBOM)](https://docs.aws.amazon.com/whitepapers/latest/practicing-continuous-integration-continuous-delivery/software-bill-of-materials-sbom.html), provides valuable insights into the composition of your software.

Generate a comprehensive inventory as part of each build. This forms a continuous record of your software's composition, enabling quick and efficient identification and management of potential vulnerabilities or risks. Tracking inventory that is machine readable enhances visibility and aids in identifying vulnerabilities and risks, enhancing the security posture of your software at scale.

Use a tool to create and manage SBOMs, centralizing them with other build artifacts for easier accessibility. Open-source tool sets provided by Open Worldwide Application Security Project ([OWASP](https://owasp.org/)) and the [Linux Foundation](https://www.linuxfoundation.org/) offer options for creating and managing SBOMs in standardized formats.

**Related information:**

* [Exporting SBOMs with Amazon Inspector](https://docs.aws.amazon.com/inspector/latest/user/sbom-export.html)

* [SPDX Becomes Internationally Recognized Standard for Software Bill of Materials](https://www.linuxfoundation.org/press/featured/spdx-becomes-internationally-recognized-standard-for-software-bill-of-materials)

* [Software Supply Chain Best Practices](https://project.linuxfoundation.org/hubfs/CNCF_SSCP_v1.pdf)

* [OWASP CycloneDX](https://owasp.org/www-project-cyclonedx/)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.SCM.9] Implement plans for deprecating and revoking outdated software components

Anti-patterns

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
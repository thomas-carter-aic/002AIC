[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.SCM.9] Implement plans for deprecating and revoking outdated software components

**Category:** RECOMMENDED

Maintaining an up-to-date and secure code base requires the proactive management of components, including removing outdated artifacts, libraries, and repositories. Not only does their removal reduce storage costs, but it also mitigates risks associated with deploying outdated or potentially vulnerable software. The removal process of outdated components should comply with the organization's data retention policies.

Develop clear plans for the deprecation and revocation of outdated components. These plans should include regular audits of the code base to identify deprecated or unused artifacts, libraries, and repositories. Establish timelines for deprecation and final removal of identified components. Communicate these plans to your development team and ensure that they are aware of the timelines.

Consider automating the removal process where feasible, for example, by using scripts or automated governance tools that support such functionality. By implementing such plans, you can streamline the code base, making it easier to manage and less prone to errors, while ensuring security and reducing the risk of system failures.

**Related information:**

* [AWS Well-Architected Cost Optimization Pillar: COST04-BP05 Enforce data retention policies](https://docs.aws.amazon.com/wellarchitected/latest/cost-optimization-pillar/cost_decomissioning_resources_data_retention.html)

* [AWS Well-Architected Sustainability Pillar: SUS02-BP03 Stop the creation and maintenance of unused assets](https://docs.aws.amazon.com/wellarchitected/latest/sustainability-pillar/sus_sus_user_a4.html)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.SCM.8] Use a versioning specification to manage software components

\[DL.SCM.10] Generate a comprehensive software inventory for each build

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
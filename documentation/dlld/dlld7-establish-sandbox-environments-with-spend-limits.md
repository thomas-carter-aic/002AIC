[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.LD.7] Establish sandbox environments with spend limits

**Category:** RECOMMENDED

Sandbox environments are dedicated spaces for developers to explore, experiment, and innovate with new technologies or ideas. Unlike development environments, which are meant for more structured day-to-day development, they allow more freedom and fewer controls, while ensuring no connectivity to internal networks or other environments.

Create a comprehensive sandbox usage policy. This policy must set clear boundaries on the kinds of data permissible with the sandbox, ensuring no leakage of sensitive information or code. Establish rules for access controls. Some environments might be tailored for individual developers, while others could serve small teams. Rules regarding network connectivity should ensure that the sandbox remains isolated, preventing any unintended interactions with other internal networks or environments. Set tagging strategies which can aid in managing automation and cost tracking. Overall, ensure that this policy makes a distinction between sandbox environments and development environments, and lays out the use cases best suited for each.

Educate developers on the sandbox usage policy, including responsible and cost-effective resource management techniques. Encourage shutting down or deleting unnecessary resources, especially when they're not in active use. Sandbox environments should be treated ephemerally, with automated governance processes managing the lifecycle to create, manage, clean up resources, and destroy sandbox environments as required.

**Related information:**

* [AWS Well-Architected Cost Optimization Pillar: COST02-BP05 Implement cost controls](https://docs.aws.amazon.com/wellarchitected/latest/cost-optimization-pillar/cost_govern_usage_controls.html)

* [Sandbox per builder or team with spend limits](https://docs.aws.amazon.com/whitepapers/latest/organizing-your-aws-environment/sandbox-ou.html#sandbox-per-builder-or-team-with-spend-limits)

* [AWS Innovation Sandbox](https://aws.amazon.com/solutions/implementations/aws-innovation-sandbox/)

* [Cloud Financial Management with AWS](https://aws.amazon.com/aws-cost-management/)

* [Sandbox Accounts for Events](https://github.com/awslabs/sandbox-accounts-for-events)

* [Best practices for creating and managing sandbox accounts in AWS](https://aws.amazon.com/blogs/mt/best-practices-creating-managing-sandbox-accounts-aws/)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.LD.6] Leverage extensible development tools

\[DL.LD.8] Generate mock datasets for local development

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
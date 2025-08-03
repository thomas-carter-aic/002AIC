[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [AG.SAD.1] Centralize and federate access with temporary credential vending

**Category:** FOUNDATIONAL

Implement a centralized subsystem for federated access and temporary credential vending to maintain secure and controlled access to your environments, workloads, and resources. By implementing a federated access solution, you can leverage your existing identity systems, provide single sign-on (SSO) capabilities, and avoid the need to maintain separate user identities across multiple systems which makes scaling in a DevOps model more tenable. Centralizing identity onboarding and permission management eliminate the inefficiencies of manual processes, reduce human error, and enable scalability as your organization grows.

Grant users and services fine-grained access to help ensure secure, granular control as they interact with resources and systems. By applying the least privilege principle, you can minimize the risk of unauthorized access and reduce the potential damage from compromised keys while retaining full control over access to resources and environments. To reduce the likelihood of keys being compromised, always vend short-lived, temporary credentials that are scoped for specific tasks to help ensure that privileges are granted only for the duration needed.

**Related information:**

* [AWS Well-Architected Cost Optimization Pillar: COST02-BP04 Implement groups and roles](https://docs.aws.amazon.com/wellarchitected/latest/cost-optimization-pillar/cost_govern_usage_groups_roles.html)

* [Security best practices in IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)

* [AWS Well-Architected Security Pillar: SEC02-BP04 Rely on a centralized identity provider](https://docs.aws.amazon.com/wellarchitected/latest/security-pillar/sec_identities_identity_provider.html)

* [IAM Identity Center](https://docs.aws.amazon.com/singlesignon/latest/userguide/what-is.html)

* [What is SSO (Single-Sign-On)?](https://aws.amazon.com/what-is/sso/)

* [Identity providers and federation](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_providers.html)


[Document Conventions](/general/latest/gr/docconventions.html)

Indicators for secure access and delegation

\[AG.SAD.2] Delegate identity and access management responsibilities

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
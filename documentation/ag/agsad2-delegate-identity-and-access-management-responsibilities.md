[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [AG.SAD.2] Delegate identity and access management responsibilities

**Category:** FOUNDATIONAL

Create a decentralized Identity and Access Management (IAM) responsibility model that enables individual teams to handle their own IAM tasks, such as creating roles and assigning permissions, as long as those teams operate within applied guardrails. This approach grants teams the autonomy to manage their roles and permissions essential for the applications they develop, encourages a culture of ownership and accountability, and enables your organization to scale its permission management effectively as it grows and embraces more DevOps practices.

Establish a set of well-defined guardrails which limit the maximum permissions a user or role can safely have. These guardrails reduce potential security risk while creating balance between allowing teams to manage their own IAM tasks and ensuring that they do not exceed the maximum permissions set.

**Related information:**

* [Security best practices in IAM](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)

* [Use permissions boundaries to delegate permissions management within an account](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#bp-permissions-boundaries)

* [Establish permissions guardrails across multiple accounts](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html#bp-permissions-guardrails)

* [Blog: Delegate permission management to developers by using IAM permissions boundaries](https://aws.amazon.com/blogs/security/delegate-permission-management-to-developers-using-iam-permissions-boundaries/)


[Document Conventions](/general/latest/gr/docconventions.html)

\[AG.SAD.1] Centralize and federate access with temporary credential vending

\[AG.SAD.3] Treat pipelines as production resources

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
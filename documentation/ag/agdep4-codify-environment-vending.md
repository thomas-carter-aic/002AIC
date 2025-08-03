[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [AG.DEP.4] Codify environment vending

**Category:** RECOMMENDED

A core benefit of the DevOps model is team autonomy and reducing cross-team dependencies. Through infrastructure as code (IaC), teams can establish and manage their environments autonomously in a self-service manner, shifting from traditional methods where operations teams would oversee these responsibilities.

By provisioning environments, and the accounts operating them, as IaC or API calls, teams are empowered with the flexibility to create environments according to their specific requirements and ways of working. Codifying the environment provisioning process provides teams with the flexibility to create both persistent and ephemeral environments based on their specific needs and workflows. In particular, this code-based approach enables the easy creation of ephemeral environments that can be automatically setup and torn down when not in use, optimizing resource utilization and cost.

Use shared libraries or services that allow teams to request and manage environments using IaC. These libraries should encapsulate best practices for environment configuration and should be designed to be used directly in deployment pipelines, enabling individual teams to manage their environments autonomously. This reduces the need for manual requests or interactions with a developer portal, as well as reduces the reliance on platform teams for provisioning and managing environments on their behalf. This approach promotes consistency and reduces overhead from cross-team collaboration.

**Related information:**

* [What is the AWS CDK?](https://docs.aws.amazon.com/cdk/v2/guide/home.html)

* [Create an AWS Proton environment](https://docs.aws.amazon.com/proton/latest/userguide/ag-create-env.html)

* [Provision and manage accounts with Account Factory](https://docs.aws.amazon.com/controltower/latest/userguide/account-factory.html)

* [Provision Accounts Through Service Catalog](https://docs.aws.amazon.com/controltower/latest/userguide/service-catalog.html)


[Document Conventions](/general/latest/gr/docconventions.html)

\[AG.DEP.3] Enable deployment to the landing zone

\[AG.DEP.5] Standardize and manage shared resources across environments

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
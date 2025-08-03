[Documentation](/index.html)[AWS Well-Architected Framework](mergers-and-acquisitions-lens.html)

[MAOPS03-BP01 Structure your organization following AWS best practices](#maops03-bp01)[MAOPS03-BP02 Merge the management accounts of both organizations](#maops03-bp02)[MAOPS03-BP03 Determine if it's appropriate to separate management accounts](#maops03-bp03)[MAOPS03-BP04 Merge logging, security, and infrastructure organizations](#maops03-bp04)[MAOPS03-BP05 Define a backup strategy for each organization](#maops03-bp05)

# MAOPS 3: What is your combined AWS Organizations strategy, and how do you handle cross-cloud governance?

With accounts in AWS Organizations, you can easily allocate resources, group accounts, and apply governance policies to accounts or groups. Buyer organization structure needs to be extendible to accommodate new organization structure. Both involved organizations should come up with the right structure to support operational excellence.

## MAOPS03-BP01 Structure your organization following AWS best practices

A well-architected multi-account strategy helps you innovate faster in AWS, while helping you meet your security and scalability needs.

## MAOPS03-BP02 Merge the management accounts of both organizations

Consolidated billing is a feature of AWS Organizations. You can use the management account of your organization to consolidate and pay for all member accounts. In consolidated billing, management accounts can also access the billing information, account information, and account activity of member accounts in their organization. This information may be used for services such as AWS Cost Explorer, which can help management accounts improve their organization’s cost performance.

## MAOPS03-BP03 Determine if it's appropriate to separate management accounts

If there is a use case to keep OUs separate, you can certainly do that with multiple management accounts. There may be few reasons to keep Organizations separate:

1. AWS GovCloud (US) or commercial cloud

2. Differing financial needs, including taxation (Europe compared to the US)

3. Differing operating scope (Systems Manager)

## MAOPS03-BP04 Merge logging, security, and infrastructure organizations

The approach covered in this pattern is suitable for customers who have multiple AWS accounts with AWS Organizations and are now encountering challenges when using AWS Control Tower, a landing zone, or account vending machine services to set up baseline guardrails in their accounts.

## MAOPS03-BP05 Define a backup strategy for each organization

Use AWS Backup to create backup plans that define how to back up your AWS resources. The rules in the plan include a variety of settings, such as backup frequency, the time window during which the backup occurs, the AWS Region containing the resources to back up, and the vault in which to store the backup.


[Document Conventions](/general/latest/gr/docconventions.html)

MAOPS 2: How do you plan to set up and govern a secure, multi-account, or multi-cloud AWS environment?

MAOPS 4: How does technical debt hamper new feature development, hosting efficiencies, or cost reductions?

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
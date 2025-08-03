[Documentation](/index.html)[AWS Well-Architected Framework](mergers-and-acquisitions-lens.html)

[MASEC02-BP01 Use an AWS-defined process to report vulnerabilities](#masec02-bp01)[MASEC02-BP02 Use AWS services with self-service within the existing management console](#masec02-bp02)[MASEC02-BP03 Use third-party security tools when necessary due to integration with on-premises resources](#masec02-bp03)[MASEC02-BP04 Migrate to a common set of tools, including partner tools from marketplace](#masec02-bp04)[MASEC02-BP05 Create a common policy for auditing and rotating credentials](#masec02-bp05)

# MASEC 2: What security tools (AWS or third-party) do you use?

Security is a shared responsibility. It is important to understand if the seller is using AWS services to find and remediate vulnerabilities, misconfigurations, and resources. Are they using third party tools to do this?

## MASEC02-BP01 Use an AWS-defined process to report vulnerabilities

AWS takes security very seriously and investigates all reported vulnerabilities (for more detail, see [AWS Cloud Security](https://aws.amazon.com/security/)).

## MASEC02-BP02 Use AWS services with self-service within the existing management console

On AWS, you can automate manual security tasks so you can shift your focus to scaling and innovating your business.

## MASEC02-BP03 Use third-party security tools when necessary due to integration with on-premises resources

Amazon Security Lake is a fully-managed security data lake service. You can use Security Lake to automatically centralize security data from AWS and third-party sources into a data lake that's stored in your AWS account. Security Lake helps you analyze security data, so you can get a more complete understanding of your security posture across the entire organization. You can also use Security Lake to improve the protection of your workloads, applications, and data.

## MASEC02-BP04 Migrate to a common set of tools, including partner tools from marketplace

The AWS Shared Responsibility Model (SRM) makes it easy to understand various choices for protecting unique AWS environment, and [access partner resources](https://aws.amazon.com/partners/featured/security/) that can help you implement end-to-end security quickly and easily.

## MASEC02-BP05 Create a common policy for auditing and rotating credentials

For human identities, you should require users to change their passwords periodically and retire access keys in favor of temporary credentials. For machine identities, rely on temporary credentials using IAM roles. For situations where this is not possible, frequent auditing and rotating access keys is necessary.


[Document Conventions](/general/latest/gr/docconventions.html)

MASEC 1: How do you plan to manage user and application identities across companies?

MASEC 3: How do you plan to maintain your data security posture?

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
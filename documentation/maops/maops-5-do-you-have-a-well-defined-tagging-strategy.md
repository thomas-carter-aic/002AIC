[Documentation](/index.html)[AWS Well-Architected Framework](mergers-and-acquisitions-lens.html)

[MAOPS05-BP01 Configure AWS resource tags](#maops05-bp01)[MAOPS05-BP02 Group applications based on tags](#maops05-bp02)[MAOPS05-BP03 Associate tags with each configured resource (during provisioning)](#maops05-bp03)[MAOPS05-BP04 Set up security based on tags](#maops05-bp04)[MAOPS05-BP05 Perform cost allocation based on tags](#maops05-bp05)

# MAOPS 5: Do you have a well-defined tagging strategy?

Tags are key and value pairs that act as metadata for organizing your AWS resources. With most AWS resources, you have the option of adding tags during creation. Tags can help you manage, identify, organize, search for, and filter resources. You can create tags to categorize resources by purpose, owner, environment, or other criteria.

## MAOPS05-BP01 Configure AWS resource tags

AWS resources can be tagged for a variety of purposes, from implementing a cost allocation strategy to supporting automation or authorizing access to AWS resources. Implementing a tagging strategy can be challenging for some organizations, owing to the number of stakeholder groups involved and considerations such as data sourcing and tag governance.

## MAOPS05-BP02 Group applications based on tags

A tag is a label that you assign to an AWS resource. A tag consists of a key and a value, both of which you define. For example, if you have two EC2 instances, you might assign both a tag key of `Stack`. But the value of `Stack` might be `Testing` for one and `Production` for the other.

## MAOPS05-BP03 Associate tags with each configured resource (during provisioning)

AWS CloudFormation provides a common language for provisioning all the infrastructure resources in your AWS environment. For AWS resources using AWS CloudFormation templates, you can use the AWS CloudFormation Resource Tags property to apply tags to supported resource types upon creation. Managing the tags as well as the resources with IaC helps create consistency.

## MAOPS05-BP04 Set up security based on tags

Organizations have varying needs and obligations to meet regarding the appropriate handling of data storage and processing. Data classification is an important precursor for several use cases, such as access control, data retention, data analysis, and compliance.

## MAOPS05-BP05 Perform cost allocation based on tags

The AWS-generated tag created by is a tag that AWS defines and applies to supported AWS resources for cost allocation purposes. User-defined tags are tags that you define, create, and apply to resources. After you have created and applied the user-defined tags, you can activate by using the AWS Cost Management Console for cost allocation tracking.


[Document Conventions](/general/latest/gr/docconventions.html)

MAOPS 4: How does technical debt hamper new feature development, hosting efficiencies, or cost reductions?

MAOPS 6: How do you plan to use key industry domain knowledge, intellectual property (like patents and algorithms), and open-source tools after an acquisition as a barrier to entry?

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
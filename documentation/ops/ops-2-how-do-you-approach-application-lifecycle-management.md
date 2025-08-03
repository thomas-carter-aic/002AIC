[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you approach application lifecycle management?

# **OPS 2:** How do you approach application lifecycle management?

Adopt lifecycle management approaches that improve the flow of changes to production with higher fidelity, fast feedback on quality, and quick bug fixing. These practices help you rapidly identify, remediate, and limit changes that impact customer experience.

## Resources

[AWS SAM template example using nested stacks](https://github.com/awslabs/realworld-serverless-application/blob/master/backend/sam/app/template.yaml?ref=wellarchitected)  
 [AWS SAM template example using parametrized environment](https://github.com/awslabs/realworld-serverless-application/blob/master/sam/app/template.yaml?ref=wellarchitected)  
 [Quick-start reference for cross-account deployments](https://aws-quickstart.s3.amazonaws.com/quickstart-trek10-serverless-enterprise-cicd/doc/serverless-cicd-for-the-enterprise-on-the-aws-cloud.pdf?ref=wellarchitected)  
 [Using dynamic references to specify template values in AWS CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/dynamic-references.html?ref=wellarchitected)  
 [Amazon Partner Blog: Multi-account deployments](https://www.stackery.io/blog/multi-account-best-practices/?ref=wellarchitected)

## Best Practices:

* **Use infrastructure as code and stages isolated in separate environments**: Use infrastructure as code and version control to enable tracking of changes and releases. Isolate development and production stages in separate environments. This reduces errors caused by manual processes and helps increase levels of control to gain confidence your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") operates as intended.

* **Use CI/CD including automated testing across separate accounts**: Automate build, deployment, testing, and rollback of the [[workload](serv.concept.workload.en.html "The set of components that together deliver business value.")](https://wa.aws.amazon.com/wat.concept.workload.en.html?ref=wellarchitected-ws) upon KPI and operational alerts. This eases troubleshooting, enables faster remediation and feedback time, and enable automatic and manual rollback/roll-forward should an alert trigger.

* **Prototype new features using temporary environments**: Use infrastructure as code to create temporary environments for new features you may need to prototype, and tear them down as you complete them. Temporary environments allows for higher fidelity when working with managed services, and increase levels of control to gain confidence your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") integrates and operates as intended.

* **Use a rollout deployment mechanism**: Use rollout deployment as opposed to all-at-once mechanisms. Rollout deployment can limit application changes to a small set of customers in production gradually while all-at-once is used for non-production systems.

* **Use configuration management**: Use configuration management systems to make and track configuration changes. These systems reduce errors caused by manual processes, reduce the level of effort to deploy changes, and help isolate configuration to business logic.

* **Review the function runtime deprecation policy**: AWS provided function runtimes follow official long-term support deprecation policies. Third-party provided runtime deprecation policy may differ from official long-term support. Consider reviewing your runtime deprecation policy and having a mechanism to report on runtimes that if deprecated may affect your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") to operate as intended.

## Improvement Plan

**Use infrastructure as code and stages isolated in separate environments**  

* Use a Serverless framework to help you execute functions locally, build and package application code, separate packaging from deployment, deploy to isolated stages in separate environments, and support secrets via configuration management systems.
  * With AWS Serverless Application Model (AWS SAM), you can use [AWS CloudFormation](serv.concept.cloudformation.en.html "A service for writing or changing templates that create and delete related AWS resources together as a unit.") parameters and stacks to parametrize environments.
  * With AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") Roles, AWS SAM can assume temporary credentials and deploy your Serverless application to separate AWS accounts.
  * AWS System Manager Parameter Store and AWS Secrets Manager have native integration with CloudFormation, and via parameters you can separate dynamic configuration from your infrastructure logic.
  

* For a large number of resources, consider breaking common functionalities such as alarms into separate infrastructure as code templates.
  * Use [AWS CloudFormation](serv.concept.cloudformation.en.html "A service for writing or changing templates that create and delete related AWS resources together as a unit.") nested stacks to help you deploy them as part of your Serverless application stack.  
[Amazon Partner Blog: Amazon CloudFormation Nested Stacks Primer](https://www.trek10.com/blog/cloudformation-nested-stacks-primer/?ref=wellarchitected)  
[Serverless Application example with Nested stacks](https://github.com/awslabs/realworld-serverless-application/wiki/CloudFormation?ref=wellarchitected)
  

**Use CI/CD including automated testing across separate accounts**  

* Use a [Continuous Integration](serv.concept.continuous-integration.en.html "Automation that is used to perform builds of software and automate tests against that software.")/[Continuous Deployment](serv.concept.continuous-deployment.en.html "Automated deployment to production which is dependent on results from testing and building. Every time a build and all the tests occur with no errors or failed tests, code is deployed automatically.") (CI/CD) pipeline solution that deploys multiple stages in isolated environments/accounts.

* Automate testing including but not limited to unit, integration, and end-to-end tests.

* Favor rollout deployments over all-at-once deployments for better resilience, and gradually learn what metrics will best determine your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.")’s health to appropriately alert on.

* Use a deployment system that supports traffic shifting as part of your pipeline, and rollback/roll-forward traffic to previous versions if an alert is triggered.  
 [Traffic shifting using AWS CodeDeploy, AWS SAM and Amazon CloudWatch Alerts](https://github.com/awslabs/serverless-application-model/blob/master/docs/safe_lambda_deployments.rst?ref=wellarchitected)

  * <u>Examples</u>

    * The order service pipeline automates unit, integration, and end-to-end tests. Deployments are rolled out to customers linearly, and are automatically rolled back should an impact to Order KPIs be observed via alerts.
  

**Prototype new features using temporary environments**  

* Use a Serverless framework to deploy temporary environments named after a feature.
  * With AWS SAM or a trusted third-party framework, deploy an [AWS CloudFormation](serv.concept.cloudformation.en.html "A service for writing or changing templates that create and delete related AWS resources together as a unit.") stack named after a feature you are working on (for example, feat-new-basket), and [tag](serv.concept.tag.en.html "Assign metadata to AWS resources to categorize and organize.") appropriately to help identify the owner.  
[AWS Multiple Account Billing Strategy](https://aws.amazon.com/answers/account-management/aws-multi-account-billing-strategy/?ref=wellarchitected)  
[Best practices to secure a newly created AWS Account](https://aws.amazon.com/answers/security/aws-secure-account-setup/?ref=wellarchitected)
  

* Implement a process to identify temporary environments that may have not been deleted over an extended period of time.
  * Identify how long a temporary environment generally remains available.
  * Include the stack’s owner contact details in [AWS CloudFormation](serv.concept.cloudformation.en.html "A service for writing or changing templates that create and delete related AWS resources together as a unit.") stack [tags](serv.concept.tag.en.html "Assign metadata to AWS resources to categorize and organize.").
  * Use [Amazon CloudWatch](serv.concept.amazoncw.en.html "A web service that enables you to monitor and manage various metrics, and configure alarm actions based on data from those metrics.") [Events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") scheduled tasks to notify and [tag](serv.concept.tag.en.html "Assign metadata to AWS resources to categorize and organize.") temporary environments for deletion, and provide a mechanism to extend its deletion should that be necessary.
  

* Prototype application code locally and test integrations directly with managed services.
  * With AWS SAM or a trusted third-party framework, you can iterate on an [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions’ code locally.
  * Use temporary credentials before invoking functions locally so that your application code can interact with a deployed managed service rather than mocks.
  * Similarly, run integration and end-to-end tests against a deployed environment via a [Continuous Integration](serv.concept.continuous-integration.en.html "Automation that is used to perform builds of software and automate tests against that software.") pipeline.  
[Invoking AWS Lambda functions locally with AWS SAM](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-using-invoke.html?ref=wellarchitected)  
[Invoking AWS Lambda functions locally with Serverless framework](https://serverless.com/framework/docs/providers/aws/cli-reference/invoke-local/?ref=wellarchitected)  
[Amazon Partner: Develop locally against cloud services with Stackery](https://docs.stackery.io/docs/workflow/local-development/?ref=wellarchitected)
  

**Use a rollout deployment mechanism**  

* For production systems, use a linear deployment strategy to gradually roll out changes to customers.
  * Publish a new version for every new [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function deployment.
  * Create Lambda aliases pointing to versions that reflect features or stable.
  * Use aliases for invoking Lambda functions, omitting will invoke the latest application code deployed that may not reflect a stable version or a desired feature.
  * Use Lambda alias’ routing configuration to enable traffic shifting by pointing to multiple versions.
    * <u>Examples</u>

      * Lambda alias named stable can be configured to point to application version 1.2 for 90% of the invocation it receives while 10% to be routed to 1.3.
  * AWS SAM natively integrates with [AWS CodeDeploy](serv.concept.awscodedeploy.en.html "A service that automates code deployments to any instance, including EC2 instances and instances running on-premises.") to provide rollout deployments for Serverless resources. It supports both linear, canary and all-at-once strategies.  
[AWS SAM rollout deployment configuration](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/automating-updates-to-serverless-apps.html?ref=wellarchitected)
  

* For high volume production systems, use a [canary deployment](serv.concept.canary-deployment.en.html "The slow rollout of a new version of an existing application.") strategy when you want to limit changes to a fixed percentage of customers for an extended period of time.  
 [Setting up canary deployments for Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/canary-release.html?ref=wellarchitected)

**Use configuration management**  

* Use environment variables for configuration options that change infrequently such as logging levels, and database connection strings.  
 [Setting up Lambda environment variables](https://docs.aws.amazon.com/lambda/latest/dg/env_variables.html?ref=wellarchitected)  
 [Storing sensitive information in environment variables](https://docs.aws.amazon.com/lambda/latest/dg/env_variables.html?ref=wellarchitected)  
 [Passing API Gateway stage-specific metadata to Lambda functions](https://docs.aws.amazon.com/apigateway/latest/developerguide/amazon-api-gateway-using-stage-variables.html?ref=wellarchitected)

* Use a configuration management system for dynamic configuration that might change frequently or contain sensitive data such as secrets.  
 [Amazon Partner Blog: AWS Lambda and Secret Management](https://epsagon.com/blog/aws-lambda-and-secret-management/?ref=wellarchitected)

**Review the function runtime deprecation policy**  

* Identify and report on runtimes that might deprecate, and their support policy.
  * Review [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") runtime support policy to understand deprecation schedule for your runtime.
  * Review AWS Personal Health Dashboard (AWS PHD) notifications. With AWS PHD, you can also automate notifications and send to custom communication channels other than your AWS Account email.
  * Use [AWS Config](serv.concept.config.en.html "A fully managed service that provides an AWS resource inventory, configuration history, and configuration change notifications for better security and governance. You can create rules that automatically check the configuration of AWS resources that AWS Config records.") to report on [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions runtime that might be near their deprecation.
  * In the [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") of being unable to migrate to newer runtimes within the deprecation schedule, use [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") Custom Runtime feature as an interim solution.  
[Running compliance and operational checks with AWS Config for AWS Lambda functions](https://docs.aws.amazon.com/en_pv/config/latest/developerguide/lambda-function-settings-check.html?ref=wellarchitected)  
[Automating AWS Personal Health Dashboard with custom notifications](https://github.com/aws/aws-health-tools?ref=wellarchitected)  
[Bringing your own runtime with AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-custom.html?ref=wellarchitected)
  
[Sign In to the Console](https://console.aws.amazon.com/console/home?nc1=f_ct&src=footer-signin-mobile)

### Learn About AWS

* [What Is AWS?](https://aws.amazon.com/what-is-aws/?nc1=f_cc)
* [What Is Cloud Computing?](https://aws.amazon.com/what-is-cloud-computing/?nc1=f_cc)
* [What Is DevOps?](https://aws.amazon.com/devops/what-is-devops/?nc1=f_cc)
* [What Is a Container?](https://aws.amazon.com/containers/?nc1=f_cc)
* [What Is a Data Lake?](https://aws.amazon.com/big-data/datalakes-and-analytics/what-is-a-data-lake/?nc1=f_cc)
* [AWS Cloud Security](https://aws.amazon.com/security/?nc1=f_cc)
* [What's New](https://aws.amazon.com/new/?nc1=f_cc)
* [Blogs](https://aws.amazon.com/blogs/?nc1=f_cc)
* [Press Releases](https://press.aboutamazon.com/press-releases/aws "Press Releases")

### Resources for AWS

* [Getting Started](https://aws.amazon.com/getting-started/?nc1=f_cc)
* [Training and Certification](https://aws.amazon.com/training/?nc1=f_cc)
* [AWS Solutions Portfolio](https://aws.amazon.com/solutions/?nc1=f_cc)
* [Architecture Center](https://aws.amazon.com/architecture/?nc1=f_cc)
* [Product and Technical FAQs](https://aws.amazon.com/faqs/?nc1=f_dr)
* [Analyst Reports](https://aws.amazon.com/resources/analyst-reports/?nc1=f_cc)
* [AWS Partner Network](https://aws.amazon.com/partners/?nc1=f_dr)

### Developers on AWS

* [Developer Center](https://aws.amazon.com/developer/?nc1=f_dr)
* [SDKs& Tools](https://aws.amazon.com/developer/tools/?nc1=f_dr)
* [.NET on AWS](https://aws.amazon.com/developer/language/net/?nc1=f_dr)
* [Python on AWS](https://aws.amazon.com/developer/language/python/?nc1=f_dr)
* [Java on AWS](https://aws.amazon.com/developer/language/java/?nc1=f_dr)
* [PHP on AWS](https://aws.amazon.com/developer/language/php/?nc1=f_cc)
* [Javascript on AWS](https://aws.amazon.com/developer/language/javascript/?nc1=f_dr)

### Help

* [Contact Us](https://aws.amazon.com/contact-us/?nc1=f_m)
* [AWS Careers](https://aws.amazon.com/careers/?nc1=f_hi)
* [File a Support Ticket](https://console.aws.amazon.com/support/home/?nc1=f_dr)
* [Knowledge Center](https://aws.amazon.com/premiumsupport/knowledge-center/?nc1=f_dr)
* [AWS Support Overview](https://aws.amazon.com/premiumsupport/?nc1=f_dr)
* [Legal](https://aws.amazon.com/legal/?nc1=f_cc)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc1=f_ct&src=default)

Amazon is an Equal Opportunity Employer: *Minority / Women / Disability / Veteran / Gender Identity / Sexual Orientation / Age.*

* Language
* [English](https://wa.aws.amazon.com/index.en.html)
* [日本語](https://wa.aws.amazon.com/index.ja.html)
* [한국어](https://wa.aws.amazon.com/index.ko.html)
* [Français](https://wa.aws.amazon.com/index.fr.html)
* [Português](https://wa.aws.amazon.com/index.pt_BR.html)
* [Deutsch](https://wa.aws.amazon.com/index.de.html)
* [Español](https://wa.aws.amazon.com/index.es.html)
* [Italiano](https://wa.aws.amazon.com/index.it.html)
* [中文 (繁體)](https://wa.aws.amazon.com/index.zh_TW.html)
* [中文 (简体)](https://wa.aws.amazon.com/index.zh_CN.html)

* [Privacy](https://aws.amazon.com/privacy/?nc1=f_pr)
* |
* [Site Terms](https://aws.amazon.com/terms/?nc1=f_pr)
* |
* &copy; 2023, Amazon Web Services, Inc. or its affiliates. All rights reserved.
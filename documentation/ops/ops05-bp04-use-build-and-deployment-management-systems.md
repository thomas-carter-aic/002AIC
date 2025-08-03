[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS05-BP04 Use build and deployment management systems

Use build and deployment management systems. These systems reduce errors caused by manual processes and reduce the level of effort to deploy changes.

In AWS, you can build continuous integration/continuous deployment (CI/CD) pipelines using services such as [AWS Developer Tools](https://aws.amazon.com/products/developer-tools/) (for example, [AWS CodeBuild](https://aws.amazon.com/codebuild/), [AWS CodePipeline](https://aws.amazon.com/codepipeline/), and [AWS CodeDeploy](https://aws.amazon.com/codedeploy/)).

**Desired outcome:** Your build and deployment management systems support your organization's continuous integration continuous delivery (CI/CD) system that provide capabilities for automating safe rollouts with the correct configurations.

**Common anti-patterns:**

* After compiling your code on your development system, you copy the executable onto your production systems and it fails to start. The local log files indicates that it has failed due to missing dependencies.

* You successfully build your application with new features in your development environment and provide the code to quality assurance (QA). It fails QA because it is missing static assets.

* On Friday, after much effort, you successfully built your application manually in your development environment including your newly coded features. On Monday, you are unable to repeat the steps that allowed you to successfully build your application.

* You perform the tests you have created for your new release. Then you spend the next week setting up a test environment and performing all the existing integration tests followed by the performance tests. The new code has an unacceptable performance impact and must be redeveloped and then retested.

**Benefits of establishing this best practice:** By providing mechanisms to manage build and deployment activities you reduce the level of effort to perform repetitive tasks, free your team members to focus on their high value creative tasks, and limit the introduction of error from manual procedures.

**Level of risk exposed if this best practice is not established:** Medium

## Implementation guidance

Build and deployment management systems are used to track and implement change, reduce errors caused by manual processes, and reduce the level of effort required for safe deployments. Fully automate the integration and deployment pipeline from code check-in through build, testing, deployment, and validation. This reduces lead time, decreases cost, encourages increased frequency of change, reduces the level of effort, and increases collaboration.

### Implementation steps

![Diagram showing a CI/CD pipeline using AWS CodePipeline and related services](/images/wellarchitected/2025-02-25/framework/images/deployment-pipeline-tooling.png)

*Diagram showing a CI/CD pipeline using AWS CodePipeline and related services*

1. Use a version control system to store and manage assets (such as documents, source code, and binary files).

2. Use CodeBuild to compile your source code, runs unit tests, and produces artifacts that are ready to deploy.

3. Use CodeDeploy as a deployment service that automates application deployments to [Amazon EC2](https://aws.amazon.com/ec2/) instances, on-premises instances, [serverless AWS Lambda functions](https://docs.aws.amazon.com/lambda/latest/dg/welcome.html), or [Amazon ECS](https://aws.amazon.com/ecs/).

4. Monitor your deployments.

## Resources

**Related best practices:**

* [OPS06-BP04 Automate testing and rollback](./ops_mit_deploy_risks_auto_testing_and_rollback.html)

**Related documents:**

* [AWS Developer Tools](https://aws.amazon.com/products/developer-tools/)

* [What is AWS CodeBuild?](https://docs.aws.amazon.com/codebuild/latest/userguide/welcome.html)

* [AWS CodeBuild](https://aws.amazon.com/codebuild/)

* [What is AWS CodeDeploy?](https://docs.aws.amazon.com/codedeploy/latest/userguide/welcome.html)

**Related videos:**

* [AWS re:Invent 2022 - AWS Well-Architected best practices for DevOps on AWS](https://youtu.be/hfXokRAyorA)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS05-BP03 Use configuration management systems

OPS05-BP05 Perform patch management

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
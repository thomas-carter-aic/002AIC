[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS05-BP05 Perform patch management

Perform patch management to gain features, address issues, and remain compliant with governance. Automate patch management to reduce errors caused by manual processes, scale, and reduce the level of effort to patch.

Patch and vulnerability management are part of your benefit and risk management activities. It is preferable to have immutable infrastructures and deploy workloads in verified known good states. Where that is not viable, patching in place is the remaining option.

[AWS Health](https://aws.amazon.com/premiumsupport/technology/aws-health/) is the authoritative source of information about planned lifecycle events and other action-required events that affect the health of your AWS Cloud resources. You should be aware of upcoming changes and updates that should be performed. Major planned lifecycle events are sent at least six months in advance.

[Amazon EC2 Image Builder](https://aws.amazon.com/image-builder/) provides pipelines to update machine images. As a part of patch management, consider [Amazon Machine Images](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) (AMIs) using an [AMI image pipeline](https://docs.aws.amazon.com/imagebuilder/latest/userguide/start-build-image-pipeline.html) or container images with a [Docker image pipeline](https://docs.aws.amazon.com/imagebuilder/latest/userguide/start-build-container-pipeline.html), while AWS Lambda provides patterns for [custom runtimes and additional libraries](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-custom.html) to remove vulnerabilities.

You should manage updates to [Amazon Machine Images](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) for Linux or Windows Server images using [Amazon EC2 Image Builder](https://aws.amazon.com/image-builder/). You can use [Amazon Elastic Container Registry (Amazon ECR)](https://docs.aws.amazon.com/AmazonECR/latest/userguide/what-is-ecr.html) with your existing pipeline to manage Amazon ECS images and manage Amazon EKS images. Lambda includes [version management features](https://docs.aws.amazon.com/lambda/latest/dg/configuration-versions.html).

Patching should not be performed on production systems without first testing in a safe environment. Patches should only be applied if they support an operational or business outcome. On AWS, you can use [AWS Systems Manager Patch Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-patch.html) to automate the process of patching managed systems and schedule the activity using [Systems Manager Maintenance Windows](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-maintenance.html).

**Desired outcome:** Your AMI and container images are patched, up-to-date, and ready for launch. You are able to track the status of all deployed images and know patch compliance. You are able to report on current status and have a process to meet your compliance needs.

**Common anti-patterns:**

* You are given a mandate to apply all new security patches within two hours resulting in multiple outages due to application incompatibility with patches.

* An unpatched library results in unintended consequences as unknown parties use vulnerabilities within it to access your workload.

* You patch the developer environments automatically without notifying the developers. You receive multiple complaints from the developers that their environment cease to operate as expected.

* You have not patched the commercial off-the-shelf software on a persistent instance. When you have an issue with the software and contact the vendor, they notify you that version is not supported and you have to patch to a specific level to receive any assistance.

* A recently released patch for the encryption software you used has significant performance improvements. Your unpatched system has performance issues that remain in place as a result of not patching.

* You are notified of a zero-day vulnerability requiring an emergency fix and you have to patch all your environments manually.

* You are not aware of critical actions needed to maintain your resources, such as mandatory version updates because you do not review upcoming planned lifecycle events and other information. You lose critical time for planning and execution, resulting in emergency changes for your teams and potential impact or unexpected downtime.

**Benefits of establishing this best practice:** By establishing a patch management process, including your criteria for patching and methodology for distribution across your environments, you can scale and report on patch levels. This provides assurances around security patching and ensure clear visibility on the status of known fixes being in place. This encourages adoption of desired features and capabilities, the rapid removal of issues, and sustained compliance with governance. Implement patch management systems and automation to reduce the level of effort to deploy patches and limit errors caused by manual processes.

**Level of risk exposed if this best practice is not established:** Medium

## Implementation guidance

Patch systems to remediate issues, to gain desired features or capabilities, and to remain compliant with governance policy and vendor support requirements. In immutable systems, deploy with the appropriate patch set to achieve the desired result. Automate the patch management mechanism to reduce the elapsed time to patch, to avoid errors caused by manual processes, and lower the level of effort to patch.

### Implementation steps

For Amazon EC2 Image Builder:

1. Using Amazon EC2 Image Builder, specify pipeline details:

   1. Create an image pipeline and name it

   2. Define pipeline schedule and time zone

   3. Configure any dependencies

2. Choose a recipe:

   1. Select existing recipe or create a new one

   2. Select image type

   3. Name and version your recipe

   4. Select your base image

   5. Add build components and add to target registry

3. Optional - define your infrastructure configuration.

4. Optional - define configuration settings.

5. Review settings.

6. Maintain recipe hygiene regularly.

For Systems Manager Patch Manager:

1. Create a patch baseline.

2. Select a patching operations method.

3. Enable compliance reporting and scanning.

## Resources

**Related best practices:**

* [OPS06-BP04 Automate testing and rollback](./ops_mit_deploy_risks_auto_testing_and_rollback.html)

**Related documents:**

* [What is Amazon EC2 Image Builder](https://docs.aws.amazon.com/imagebuilder/latest/userguide/what-is-image-builder.html)

* [Create an image pipeline using the Amazon EC2 Image Builder](https://docs.aws.amazon.com/imagebuilder/latest/userguide/start-build-image-pipeline.html)

* [Create a container image pipeline](https://docs.aws.amazon.com/imagebuilder/latest/userguide/start-build-container-pipeline.html)

* [AWS Systems Manager Patch Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-patch.html)

* [Working with Patch Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/patch-manager-console.html)

* [Working with patch compliance reports](https://docs.aws.amazon.com/systems-manager/latest/userguide/patch-manager-compliance-reports.html)

* [AWS Developer Tools](https://aws.amazon.com/products/developer-tools)

**Related videos:**

* [CI/CD for Serverless Applications on AWS](https://www.youtube.com/watch?v=tEpx5VaW4WE)

* [Design with Ops in Mind](https://youtu.be/uh19jfW7hw4)

  **Related examples:**

* [AWS Systems Manager Patch Manager tutorials](https://docs.aws.amazon.com/systems-manager/latest/userguide/patch-manager-tutorials.html)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS05-BP04 Use build and deployment management systems

OPS05-BP06 Share design standards

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
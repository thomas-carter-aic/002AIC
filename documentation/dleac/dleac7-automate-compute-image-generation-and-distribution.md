[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.EAC.7] Automate compute image generation and distribution

**Category:** OPTIONAL

The management of compute images, including containers and machine images, can be optimized and made more reliable through a code-driven approach. Compute images generally include a base image, libraries, environment variables, application code, and configuration files. Similar to other forms of infrastructure as code (IaC), compute images can be codified, stored in version control systems, tested, and distributed as part of the development lifecycle.

Establish automated pipelines for building, testing, and distributing compute images. The build stage creates the image based on its code definition, the *test* stage validates the functionality and security compliance of the image, and the *distribution* stage ensures the image is readily available for teams to use in their environments and workloads. Updates to the images should be automated, accounting for software patches, security enhancements, and other modifications.

Given the diverse range of applications and infrastructure requirements, especially when using managed cloud-based services, not all organizations or workloads necessitate using dedicated compute images or codifying them.

**Related information:**

* [Amazon EC2 Image Builder](https://aws.amazon.com/image-builder/)

* [AWS Deployment Pipeline Reference Architecture](https://aws-samples.github.io/aws-deployment-pipeline-reference-architecture)

* [What is AWS App2Container?](https://docs.aws.amazon.com/app2container/latest/UserGuide/what-is-a2c.html)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.EAC.6] Use general-purpose programming languages to generate Infrastructure-as-Code

Anti-patterns

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
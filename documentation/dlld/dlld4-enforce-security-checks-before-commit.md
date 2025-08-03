[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.LD.4] Enforce security checks before commit

**Category:** FOUNDATIONAL

Pre-commit hooks can be an effective tool for maintaining security best practices. These hooks can help in the early detection of potential security risks, such as exposed sensitive data or publishing code to untrusted repositories. At a minimum, use pre-commit hooks to identify hidden secrets, like passwords and access keys, before code is published to a shared repository. When discovering secrets, the code push should fail immediately—effectively preventing a security incident from occurring.

Select security tools compatible with your chosen programming languages and customize them to uphold your specific governance and compliance requirements. It is best to integrate these security tools into pre-commit hooks, integrated development environments (IDEs), and continuous integration pipelines so that changes are continuously checked before code is committed into a shared repository.

**Related information:**

* [Security in every stage of CI/CD pipeline: Pre-commit hooks](https://docs.aws.amazon.com/whitepapers/latest/practicing-continuous-integration-continuous-delivery/security-in-every-stage-of-cicd-pipeline.html#pre-commit-hooks)

* [Security scans - CodeWhisperer](https://docs.aws.amazon.com/codewhisperer/latest/userguide/security-scans.html)

* [Pre-commit](https://pre-commit.com/)

* [Husky](https://typicode.github.io/husky/)

* [Gitleaks](https://github.com/gitleaks/gitleaks)

* [GitGuardian](https://docs.gitguardian.com/ggshield-docs/integrations/git-hooks/pre-commit)

* [AWS-IA opinionated pre-commit hooks](https://github.com/aws-ia/pre-commit-configs)

* [Blog: Extend your pre-commit hooks with AWS CloudFormation Guard](https://aws.amazon.com/blogs/security/extend-your-pre-commit-hooks-with-aws-cloudformation-guard/)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.LD.3] Commit local changes early and often

\[DL.LD.5] Enforce coding standards before commit

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
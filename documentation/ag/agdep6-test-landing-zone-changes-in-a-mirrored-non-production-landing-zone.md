[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [AG.DEP.6] Test landing zone changes in a mirrored non-production landing zone

**Category:** RECOMMENDED

Changes to landing zones can have significant impacts across teams and processes because it is consumed by many teams in an organization. To minimize the risk of potential failures when making changes to the landing zone, platform teams should follow similar practices seen in the development lifecycle, including thorough testing and validation in a dedicated environment before rolling out to production.

When making changes to a landing zone, establish mirrored landing zones for testing changes before deploying to the production landing zone. This allows for changes to be validated without affecting the production environment. Use deployment pipelines to promote, validate, and deploy changes between the mirrored and production landing zones, performing extensive testing and validation at each stage.

Overall, this practice promotes safer changes to the production landing zone which has the potential to impact many teams in the organization. Clearly communicate with those teams before rolling out changes to the production landing zone so that they are informed of imminent changes, potential impacts to their environments and systems, and the projected timeline.

**Related information:**

* [Multiple organizations: Test changes to your overall AWS environment](https://docs.aws.amazon.com/whitepapers/latest/organizing-your-aws-environment/multiple-organizations.html)


[Document Conventions](/general/latest/gr/docconventions.html)

\[AG.DEP.5] Standardize and manage shared resources across environments

\[AG.DEP.7] Utilize metadata for scalable environment management

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
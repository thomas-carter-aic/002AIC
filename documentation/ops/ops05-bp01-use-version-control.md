[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS05-BP01 Use version control

Use version control to activate tracking of changes and releases.

Many AWS services offer version control capabilities. Use a revision or [source control](https://aws.amazon.com/devops/source-control/) system such as [Git](https://aws.amazon.com/devops/source-control/git/) to manage code and other artifacts such as version-controlled [AWS CloudFormation](https://aws.amazon.com/cloudformation/) templates of your infrastructure.

**Desired outcome:** Your teams collaborate on code. When merged, the code is consistent and no changes are lost. Errors are easily reverted through correct versioning.

**Common anti-patterns:**

* You have been developing and storing your code on your workstation. You have had an unrecoverable storage failure on the workstation and your code is lost.

* After overwriting the existing code with your changes, you restart your application and it is no longer operable. You are unable to revert the change.

* You have a write lock on a report file that someone else needs to edit. They contact you asking that you stop work on it so that they can complete their tasks.

* Your research team has been working on a detailed analysis that shapes your future work. Someone has accidentally saved their shopping list over the final report. You are unable to revert the change and have to recreate the report.

**Benefits of establishing this best practice:** By using version control capabilities you can easily revert to known good states and previous versions, and limit the risk of assets being lost.

**Level of risk exposed if this best practice is not established:** High

## Implementation guidance

Maintain assets in version controlled repositories. Doing so supports tracking changes, deploying new versions, detecting changes to existing versions, and reverting to prior versions (for example, rolling back to a known good state in the event of a failure). Integrate the version control capabilities of your configuration management systems into your procedures.

## Resources

**Related best practices:**

* [OPS05-BP04 Use build and deployment management systems](./ops_dev_integ_build_mgmt_sys.html)

**Related videos:**

* [AWS re:Invent 2023 - How Lockheed Martin builds software faster, powered by DevSecOps](https://www.youtube.com/watch?v=Q1OSyxYkl5w)

* [AWS re:Invent 2023 - How GitHub operationalizes AI for team collaboration and productivity](https://www.youtube.com/watch?v=cOVvGaiusOI)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS 5. How do you reduce defects, ease remediation, and improve flow into production?

OPS05-BP02 Test and validate changes

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
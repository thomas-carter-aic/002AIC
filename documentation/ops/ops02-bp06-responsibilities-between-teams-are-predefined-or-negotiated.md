[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS02-BP06 Responsibilities between teams are predefined or negotiated

Have defined or negotiated agreements between teams describing how they work with and support each other (for example, response times, service level objectives, or service-level agreements). Inter-team communications channels are documented. Understanding the impact of the teams’ work on business outcomes and the outcomes of other teams and organizations informs the prioritization of their tasks and helps them respond appropriately.

When responsibility and ownership are undefined or unknown, you are at risk of both not addressing necessary activities in a timely fashion and of redundant and potentially conflicting efforts emerging to address those needs.

**Desired outcome:**

* Inter-team working or support agreements are agreed to and documented.

* Teams that support or work with each other have defined communication channels and response expectations.

**Common anti-patterns:**

* An issue occurs in production and two separate teams start troubleshooting independent of each other. Their siloed efforts extend the outage.

* The operations team needs assistance from the development team but there is no agreed to response time. The request is stuck in the backlog.

**Benefits of establishing this best practice:**

* Teams know how to interact and support each other.

* Expectations for responsiveness are known.

* Communications channels are clearly defined.

**Level of risk exposed if this best practice is not established:** Low

## Implementation guidance

Implementing this best practice means that there is no ambiguity about how teams work with each other. Formal agreements codify how teams work together or support each other. Inter-team communication channels are documented.

**Customer example**

AnyCompany Retail’s SRE team has a service level agreement with their development team. Whenever the development team makes a request in their ticketing system, they can expect a response within fifteen minutes. If there is a site outage, the SRE team takes lead in the investigation with support from the development team.

**Implementation steps**

1. Working with stakeholders across your organization, develop agreements between teams based on processes and procedures.

   1. If a process or procedure is shared between two teams, develop a runbook on how the teams will work together.

   2. If there are dependencies between teams, agree to a response SLA for requests.

2. Document responsibilities in your knowledge management system.

**Level of effort for the implementation plan:** Medium. If there are no existing agreements between teams, it can take effort to come to agreement with stakeholders across your organization.

## Resources

**Related best practices:**

* [OPS02-BP02 Processes and procedures have identified owners](./ops_ops_model_def_proc_owners.html) - Process ownership must be identified before setting agreements between teams.

* [OPS02-BP03 Operations activities have identified owners responsible for their performance](./ops_ops_model_def_activity_owners.html) - Operations activities ownership must be identified before setting agreements between teams.

**Related documents:**

* [AWS Executive Insights - Empowering Innovation with the Two-Pizza Team](https://aws.amazon.com/executive-insights/content/amazon-two-pizza-team/)

* [Introduction to DevOps on AWS - Two-Pizza Teams](https://docs.aws.amazon.com/whitepapers/latest/introduction-devops-aws/two-pizza-teams.html)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS02-BP05 Mechanisms exist to request additions, changes, and exceptions

OPS 3. How does your organizational culture support your business outcomes?

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
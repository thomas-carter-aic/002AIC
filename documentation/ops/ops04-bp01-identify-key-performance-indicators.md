[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS04-BP01 Identify key performance indicators

Implementing observability in your workload starts with understanding its state and making data-driven decisions based on business requirements. One of the most effective ways to ensure alignment between monitoring activities and business objectives is by defining and monitoring key performance indicators (KPIs).

**Desired outcome:** Efficient observability practices that are tightly aligned with business objectives, ensuring that monitoring efforts are always in service of tangible business outcomes.

**Common anti-patterns:**

* Undefined KPIs: Working without clear KPIs can lead to monitoring too much or too little, missing vital signals.

* Static KPIs: Not revisiting or refining KPIs as the workload or business objectives evolve.

* Misalignment: Focusing on technical metrics that don’t correlate directly with business outcomes or are harder to correlate with real-world issues.

**Benefits of establishing this best practice:**

* Ease of issue identification: Business KPIs often surface issues more clearly than technical metrics. A dip in a business KPI can pinpoint a problem more effectively than sifting through numerous technical metrics.

* Business alignment: Ensures that monitoring activities directly support business objectives.

* Efficiency: Prioritize monitoring resources and attention on metrics that matter.

* Proactivity: Recognize and address issues before they have broader business implications.

**Level of risk exposed if this best practice is not established:** High

## Implementation guidance

To effectively define workload KPIs:

1. **Start with business outcomes:** Before diving into metrics, understand the desired business outcomes. Is it increased sales, higher user engagement, or faster response times?

2. **Correlate technical metrics with business objectives:** Not all technical metrics have a direct impact on business outcomes. Identify those that do, but it's often more straightforward to identify an issue using a business KPI.

3. **Use [Amazon CloudWatch](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/WhatIsCloudWatch.html):** Employ CloudWatch to define and monitor metrics that represent your KPIs.

4. **Regularly review and update KPIs:** As your workload and business evolve, keep your KPIs relevant.

5. **Involve stakeholders:** Involve both technical and business teams in defining and reviewing KPIs.

**Level of effort for the implementation plan:** Medium

## Resources

**Related best practices:**

* [OPS04-BP02 Implement application telemetry](./ops_observability_application_telemetry.html)

* [OPS04-BP03 Implement user experience telemetry](./ops_observability_customer_telemetry.html)

* [OPS04-BP04 Implement dependency telemetry](./ops_observability_dependency_telemetry.html)

* [OPS04-BP05 Implement distributed tracing](./ops_observability_dist_trace.html)

**Related documents:**

* [AWS Observability Best Practices](https://aws-observability.github.io/observability-best-practices/)

* [CloudWatch User Guide](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/WhatIsCloudWatch.html)

* [AWS Observability Skill Builder Course](https://explore.skillbuilder.aws/learn/course/external/view/elearning/14688/aws-observability)

**Related videos:**

* [Developing an observability strategy](https://www.youtube.com/watch?v=Ub3ATriFapQ)

**Related examples:**

* [One Observability Workshop](https://catalog.workshops.aws/observability/en-US)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS 4. How do you implement observability in your workload?

OPS04-BP02 Implement application telemetry

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>
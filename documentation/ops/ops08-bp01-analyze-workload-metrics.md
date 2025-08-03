[Documentation](/index.html)[Framework](welcome.html)

[Implementation guidance](#implementation-guidance)[Resources](#resources)

# OPS08-BP01 Analyze workload metrics

After implementing application telemetry, regularly analyze the collected metrics. While latency, requests, errors, and capacity (or quotas) provide insights into system performance, it's vital to prioritize the review of business outcome metrics. This ensures you're making data-driven decisions aligned with your business objectives.

**Desired outcome:** Accurate insights into workload performance that drive data-informed decisions, ensuring alignment with business objectives.

**Common anti-patterns:**

* Analyzing metrics in isolation without considering their impact on business outcomes.

* Over-reliance on technical metrics while sidelining business metrics.

* Infrequent review of metrics, missing out on real-time decision-making opportunities.

**Benefits of establishing this best practice:**

* Enhanced understanding of the correlation between technical performance and business outcomes.

* Improved decision-making process informed by real-time data.

* Proactive identification and mitigation of issues before they affect business outcomes.

**Level of risk exposed if this best practice is not established:** Medium

## Implementation guidance

Leverage tools like Amazon CloudWatch to perform metric analysis. AWS services such as CloudWatch anomaly detection and Amazon DevOps Guru can be used to detect anomalies, especially when static thresholds are unknown or when patterns of behavior are more suited for anomaly detection.

### Implementation steps

1. **Analyze and review:** Regularly review and interpret your workload metrics.

   1. Prioritize business outcome metrics over purely technical metrics.

   2. Understand the significance of spikes, drops, or patterns in your data.

2. **Utilize Amazon CloudWatch:** Use Amazon CloudWatch for a centralized view and deep-dive analysis.

   1. Configure CloudWatch dashboards to visualize your metrics and compare them over time.

   2. Use [percentiles in CloudWatch](https://aws-observability.github.io/observability-best-practices/guides/operational/business/sla-percentile/) to get a clear view of metric distribution, which can help in defining SLAs and understanding outliers.

   3. Set up [CloudWatch anomaly detection](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Anomaly_Detection.html) to identify unusual patterns without relying on static thresholds.

   4. Implement [CloudWatch cross-account observability](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Unified-Cross-Account.html) to monitor and troubleshoot applications that span multiple accounts within a Region.

   5. Use [CloudWatch Metric Insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/query_with_cloudwatch-metrics-insights.html) to query and analyze metric data across accounts and Regions, identifying trends and anomalies.

   6. Apply [CloudWatch Metric Math](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/using-metric-math.html) to transform, aggregate, or perform calculations on your metrics for deeper insights.

3. **Employ Amazon DevOps Guru:** Incorporate [Amazon DevOps Guru](https://aws.amazon.com/devops-guru/) for its machine learning-enhanced anomaly detection to identify early signs of operational issues for your serverless applications and remediate them before they impact your customers.

4. **Optimize based on insights:** Make informed decisions based on your metric analysis to adjust and improve your workloads.

**Level of effort for the Implementation Plan:** Medium

## Resources

**Related best practices:**

* [OPS04-BP01 Identify key performance indicators](./ops_observability_identify_kpis.html)

* [OPS04-BP02 Implement application telemetry](./ops_observability_application_telemetry.html)

**Related documents:**

* [The Wheel Blog - Emphasizing the importance of continually reviewing metrics](https://aws.amazon.com/blogs/opensource/the-wheel/)

* [Percentile are important](https://aws-observability.github.io/observability-best-practices/guides/operational/business/sla-percentile/)

* [Using AWS Cost Anomaly Detection](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch_Anomaly_Detection.html)

* [CloudWatch cross-account observability](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Unified-Cross-Account.html)

* [Query your metrics with CloudWatch Metrics Insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/query_with_cloudwatch-metrics-insights.html)

**Related videos:**

* [Enable Cross-Account Observability in Amazon CloudWatch](https://www.youtube.com/watch?v=lUaDO9dqISc)

* [Introduction to Amazon DevOps Guru](https://www.youtube.com/watch?v=2uA8q-8mTZY)

* [Continuously Analyze Metrics using AWS Cost Anomaly Detection](https://www.youtube.com/watch?v=IpQYBuay5OE)

**Related examples:**

* [One Observability Workshop](https://catalog.workshops.aws/observability/en-US/intro)

* [Gaining operation insights with AIOps using Amazon DevOps Guru](https://catalog.us-east-1.prod.workshops.aws/workshops/f92df379-6add-4101-8b4b-38b788e1222b/en-US)


[Document Conventions](/general/latest/gr/docconventions.html)

OPS 8. How do you utilize workload observability in your organization?

OPS08-BP02 Analyze workload logs

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>